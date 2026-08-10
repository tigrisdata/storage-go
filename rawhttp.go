package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
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
