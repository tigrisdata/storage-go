package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	reqURL := fmt.Sprintf("%s/%s?bundle", c.baseEndpoint(), in.Bucket)

	body, err := json.Marshal(bundleRequestBody{Keys: in.Keys})
	if err != nil {
		return nil, fmt.Errorf("storage: BundleObjects: failed to marshal keys: %w", err)
	}

	resp, err := c.doSignedRequest(ctx, http.MethodPost, reqURL, map[string]string{
		"Content-Type":                "application/json",
		"X-Tigris-Bundle-Format":      BundleFormatTar,
		"X-Tigris-Bundle-Compression": compression,
		"X-Tigris-Bundle-On-Error":    onError,
	}, body)
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
