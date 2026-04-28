package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	// BundleFormatTar is the tar archive format for bundle responses.
	BundleFormatTar = "tar"

	// BundleCompressionNone disables compression (default).
	BundleCompressionNone = "none"
	// BundleCompressionGzip enables gzip compression.
	BundleCompressionGzip = "gzip"
	// BundleCompressionZstd enables zstd compression.
	BundleCompressionZstd = "zstd"

	// BundleOnErrorSkip silently omits missing objects from the archive (default).
	BundleOnErrorSkip = "skip"
	// BundleOnErrorFail returns an error if any object is missing.
	BundleOnErrorFail = "fail"
)

// bundleHTTPClient is reused across calls. No overall timeout — the caller's
// context controls cancellation, which avoids cutting off streaming reads.
var bundleHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}

// BundleObjectsInput is the input for a BundleObjects request.
type BundleObjectsInput struct {
	// Bucket is the name of the bucket containing the objects. Required.
	Bucket string

	// Keys is the list of object keys to include in the bundle. Required.
	// Maximum 5,000 keys per request.
	Keys []string

	// Compression sets the compression algorithm for the response.
	// Valid values: "none" (default), "gzip", "zstd".
	Compression string

	// OnError controls behavior when objects are missing.
	// "skip" (default): omit missing objects and append __bundle_errors.json to the tar.
	// "fail": return an error before streaming if any object is missing.
	OnError string
}

// BundleObjectsOutput is the response from a BundleObjects request.
//
// The Body contains a streaming tar archive. Callers are responsible for closing Body.
// Use archive/tar to iterate entries:
//
//	tr := tar.NewReader(output.Body)
//	for {
//	    hdr, err := tr.Next()
//	    if err == io.EOF { break }
//	    // process hdr.Name, tr
//	}
//
// If compression was requested, wrap Body with the appropriate decompressor first:
//
//	gz, _ := gzip.NewReader(output.Body)
//	tr := tar.NewReader(gz)
type BundleObjectsOutput struct {
	// Body is the streaming tar archive. Must be closed by the caller.
	Body io.ReadCloser

	// ContentType is the response Content-Type (e.g. "application/x-tar", "application/gzip").
	ContentType string

	// StatusCode is the HTTP status code of the response.
	StatusCode int
}

type bundleRequestBody struct {
	Keys []string `json:"keys"`
}

// BundleObjects fetches multiple objects from a bucket as a streaming tar archive
// in a single HTTP request.
//
// This is a Tigris extension to the S3 API, designed for ML training workloads
// that need to fetch thousands of objects per batch without per-object HTTP overhead.
//
// The caller is responsible for closing the returned Body.
func (c *Client) BundleObjects(ctx context.Context, in *BundleObjectsInput) (*BundleObjectsOutput, error) {
	if in.Bucket == "" {
		return nil, fmt.Errorf("storage: BundleObjects: bucket is required")
	}
	if len(in.Keys) == 0 {
		return nil, fmt.Errorf("storage: BundleObjects: at least one key is required")
	}

	compression := in.Compression
	if compression == "" {
		compression = BundleCompressionNone
	}

	onError := in.OnError
	if onError == "" {
		onError = BundleOnErrorSkip
	}

	opts := c.Client.Options()

	endpoint := GlobalEndpoint
	if opts.BaseEndpoint != nil {
		endpoint = *opts.BaseEndpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")

	reqURL := fmt.Sprintf("%s/%s?bundle", endpoint, in.Bucket)

	body, err := json.Marshal(bundleRequestBody{Keys: in.Keys})
	if err != nil {
		return nil, fmt.Errorf("storage: BundleObjects: failed to marshal keys: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("storage: BundleObjects: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tigris-Bundle-Format", BundleFormatTar)
	req.Header.Set("X-Tigris-Bundle-Compression", compression)
	req.Header.Set("X-Tigris-Bundle-On-Error", onError)

	// Sign request with SigV4.
	if opts.Credentials != nil {
		creds, err := opts.Credentials.Retrieve(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage: BundleObjects: failed to retrieve credentials: %w", err)
		}

		payloadHash := sha256Hex(body)
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)

		signer := v4.NewSigner()
		region := opts.Region
		if region == "" {
			region = "auto"
		}

		err = signer.SignHTTP(ctx, creds, req, payloadHash, "s3", region, time.Now())
		if err != nil {
			return nil, fmt.Errorf("storage: BundleObjects: failed to sign request: %w", err)
		}
	}

	resp, err := bundleHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storage: BundleObjects: request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("storage: BundleObjects: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	return &BundleObjectsOutput{
		Body:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
