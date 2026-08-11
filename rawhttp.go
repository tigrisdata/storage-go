package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go"
)

// tigrisHTTPClient is reused across the raw-HTTP helpers (bundle, soft delete
// restore/list/patch) for the Tigris-specific endpoints that the S3 SDK cannot
// express. No overall timeout — the caller's context controls cancellation,
// which avoids cutting off streaming reads.
var tigrisHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}

// baseEndpoint returns the configured endpoint with any trailing slash trimmed,
// falling back to the global endpoint when none is set.
func (c *Client) baseEndpoint() string {
	endpoint := GlobalEndpoint
	if opts := c.Client.Options(); opts.BaseEndpoint != nil {
		endpoint = *opts.BaseEndpoint
	}
	return strings.TrimRight(endpoint, "/")
}

// doSignedRequest builds, SigV4-signs, and sends a raw HTTP request to a Tigris
// endpoint. It is the shared transport for Tigris extensions that the S3 SDK
// cannot express (object bundling, soft delete restore/list/patch).
//
// The request is signed with the client's credentials when they are available.
// The raw response is returned unmodified; callers are responsible for checking
// the status code and closing the body.
func (c *Client) doSignedRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	opts := c.Client.Options()
	if opts.Credentials != nil {
		creds, err := opts.Credentials.Retrieve(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
		}

		payloadHash := sha256Hex(body)
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)

		region := opts.Region
		if region == "" {
			region = "auto"
		}

		// DisableURIPathEscaping must be set for the same reason the S3 client
		// sets it in newDefaultV4Signer: the request path is already
		// percent-encoded, and the signer would otherwise escape it a second
		// time. That double-encoded canonical URI would not match the
		// single-encoded one Tigris computes, so any key holding a space, "%",
		// "+", "#", "?", or a non-ASCII byte would fail with
		// SignatureDoesNotMatch.
		signer := v4.NewSigner(func(o *v4.SignerOptions) {
			o.DisableURIPathEscaping = true
		})

		if err := signer.SignHTTP(ctx, creds, req, payloadHash, "s3", region, time.Now()); err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
	}

	// Honor a custom HTTP client (custom transport, TLS config, proxy, tracing)
	// configured on the underlying S3 client. Fall back to the shared client.
	if opts.HTTPClient != nil {
		return opts.HTTPClient.Do(req)
	}

	return tigrisHTTPClient.Do(req)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// APIError is returned when Tigris rejects one of the raw-HTTP requests this
// package sends for the endpoints the S3 SDK cannot express.
//
// It implements smithy.APIError, so errors.As reads it the same way it reads
// errors from the embedded *s3.Client:
//
//	var apiErr smithy.APIError
//	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchBucket" {
//	    // the bucket is gone
//	}
//
// The HTTP status code is available from the StatusCode field and from
// HTTPStatusCode, which is the accessor the AWS SDK response errors expose.
type APIError struct {
	// Op is the name of the method that failed, such as "RestoreBucket".
	Op string
	// StatusCode is the HTTP status code of the response.
	StatusCode int
	// Code is the error code from the response body, such as "NoSuchBucket". It
	// is empty when the body is not an S3-style error document.
	Code string
	// Message is the error message from the response body. It holds the raw body
	// when the body is not an S3-style error document.
	Message string
	// RequestID identifies the failed request server-side. Give it to support
	// when you report a problem. Optional.
	RequestID string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "storage: %s: HTTP %d", e.Op, e.StatusCode)
	if e.Code != "" {
		fmt.Fprintf(&b, ": %s", e.Code)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	return b.String()
}

// ErrorCode implements smithy.APIError.
func (e *APIError) ErrorCode() string { return e.Code }

// ErrorMessage implements smithy.APIError.
func (e *APIError) ErrorMessage() string { return e.Message }

// ErrorFault implements smithy.APIError. A 4xx status is the fault of the
// client, a 5xx status is the fault of the server.
func (e *APIError) ErrorFault() smithy.ErrorFault {
	switch {
	case e.StatusCode >= 400 && e.StatusCode < 500:
		return smithy.FaultClient
	case e.StatusCode >= 500:
		return smithy.FaultServer
	default:
		return smithy.FaultUnknown
	}
}

// HTTPStatusCode returns the HTTP status code of the response. It matches the
// accessor on the response errors of the AWS SDK, so a caller can read the
// status through one interface for both error sources.
func (e *APIError) HTTPStatusCode() int { return e.StatusCode }

// errorResponse mirrors the S3-style XML error document that Tigris returns on
// the raw-HTTP endpoints.
type errorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	RequestID string   `xml:"RequestId"`
}

// httpError reads a bounded amount of an error response body and converts it
// into an *APIError.
func httpError(resp *http.Response, op string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	out := &APIError{
		Op:         op,
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Amz-Request-Id"),
	}

	var parsed errorResponse
	if err := xml.Unmarshal(body, &parsed); err == nil {
		out.Code = parsed.Code
		out.Message = parsed.Message
		if parsed.RequestID != "" {
			out.RequestID = parsed.RequestID
		}
	}

	// A proxy or a gateway can answer with something that is not an S3 error
	// document. Keep whatever the server said so the message is not empty.
	if out.Code == "" && out.Message == "" {
		out.Message = strings.TrimSpace(string(body))
	}

	return out
}
