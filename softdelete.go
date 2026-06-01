package storage

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

// Sentinel errors returned when a required argument is missing. They are wrapped
// with the calling method's name, so callers can match them with errors.Is.
var (
	// ErrMissingBucket is returned when a required bucket name is empty.
	ErrMissingBucket = errors.New("bucket is required")
	// ErrMissingKey is returned when a required object key is empty.
	ErrMissingKey = errors.New("key is required")
	// ErrMissingVersionID is returned when a required version ID is empty.
	ErrMissingVersionID = errors.New("version ID is required")
)

// CreateBucketWithSoftDelete creates a bucket with soft delete enabled. Deleting
// an object or the bucket then moves it into a recoverable soft-deleted state for
// the retention window instead of removing it immediately.
//
// Pass retentionDays as 0 to use the default 7-day window, or a value between 7
// and 90 to set a custom window. Values outside the 7-90 range are rejected by
// the server.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) CreateBucketWithSoftDelete(ctx context.Context, in *s3.CreateBucketInput, retentionDays int, opts ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if retentionDays > 0 {
		opts = append(opts, tigrisheaders.WithSoftDelete(retentionDays))
	} else {
		opts = append(opts, tigrisheaders.WithSoftDelete())
	}

	return c.Client.CreateBucket(ctx, in, opts...)
}

// ForceDeleteBucket deletes a bucket even when it is not empty.
//
// If the bucket has soft delete enabled, it is moved to a recoverable
// soft-deleted state for the retention window instead of being permanently
// removed; use RestoreBucket to recover it. Otherwise the bucket and its
// contents are permanently deleted.
//
// This is a dangerous operation. Do not use this unless you are aware of
// the consequences of your actions. Support will not be able to help you
// recover any buckets or objects deleted in this way.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) ForceDeleteBucket(ctx context.Context, in *s3.DeleteBucketInput, opts ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	opts = append(opts, tigrisheaders.WithForceDelete())

	return c.Client.DeleteBucket(ctx, in, opts...)
}

// PermanentlyDeleteObject permanently removes a specific soft-deleted object
// version, bypassing the retention window. The version is gone for good and
// cannot be restored afterwards.
//
// versionID identifies the soft-deleted version to purge, as returned by
// ListSoftDeletedObjects, and is required: an empty versionID would target the
// latest version and create a new soft-delete marker rather than purging a
// version, the opposite of this method's intent.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) PermanentlyDeleteObject(ctx context.Context, bucket, key, versionID string, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if bucket == "" {
		return nil, fmt.Errorf("storage: PermanentlyDeleteObject: %w", ErrMissingBucket)
	}
	if key == "" {
		return nil, fmt.Errorf("storage: PermanentlyDeleteObject: %w", ErrMissingKey)
	}
	if versionID == "" {
		return nil, fmt.Errorf("storage: PermanentlyDeleteObject: %w", ErrMissingVersionID)
	}

	opts = append(opts, tigrisheaders.WithHeader("X-Tigris-Soft-Delete", "true"))

	return c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: aws.String(versionID),
	}, opts...)
}

// SoftDeletedObject describes a single soft-deleted object version returned by
// ListSoftDeletedObjects.
type SoftDeletedObject struct {
	// Key is the object key.
	Key string
	// VersionID identifies this soft-deleted version. Use it with
	// RestoreSoftDeletedObject or PermanentlyDeleteObject.
	VersionID string
	// Size is the size of the object in bytes.
	Size int64
	// ETag is the entity tag of the object.
	ETag string
	// SoftDeleted reports whether the version is soft-deleted.
	SoftDeleted bool
	// LastModified is the time the object was soft-deleted.
	LastModified time.Time
}

// ListSoftDeletedObjectsInput is the input for a ListSoftDeletedObjects request.
type ListSoftDeletedObjectsInput struct {
	// Bucket is the name of the bucket to list. Required.
	Bucket string
	// Prefix limits the response to keys that begin with the prefix. Optional.
	Prefix string
	// KeyMarker continues a truncated listing from NextKeyMarker. Optional.
	KeyMarker string
	// VersionIDMarker continues a truncated listing from NextVersionIDMarker. Optional.
	VersionIDMarker string
	// MaxKeys limits the number of versions returned. Optional; 0 uses the server default.
	MaxKeys int32
}

// ListSoftDeletedObjectsOutput is the response from a ListSoftDeletedObjects request.
type ListSoftDeletedObjectsOutput struct {
	// Objects is the list of soft-deleted object versions.
	Objects []SoftDeletedObject
	// IsTruncated reports whether more results are available beyond this page.
	IsTruncated bool
	// NextKeyMarker is the key marker to pass as KeyMarker for the next page.
	NextKeyMarker string
	// NextVersionIDMarker is the version-id marker to pass as VersionIDMarker for the next page.
	NextVersionIDMarker string
}

// listVersionsResult mirrors the S3 ListVersionsResult XML response, augmented
// with the Tigris-specific SoftDeleted/Size/ETag fields on delete markers that
// the AWS SDK does not model.
type listVersionsResult struct {
	XMLName             xml.Name            `xml:"ListVersionsResult"`
	IsTruncated         bool                `xml:"IsTruncated"`
	NextKeyMarker       string              `xml:"NextKeyMarker"`
	NextVersionIDMarker string              `xml:"NextVersionIdMarker"`
	DeleteMarkers       []deleteMarkerEntry `xml:"DeleteMarker"`
}

type deleteMarkerEntry struct {
	Key          string    `xml:"Key"`
	VersionID    string    `xml:"VersionId"`
	Size         int64     `xml:"Size"`
	ETag         string    `xml:"ETag"`
	SoftDeleted  bool      `xml:"SoftDeleted"`
	LastModified time.Time `xml:"LastModified"`
}

// ListSoftDeletedObjects lists the soft-deleted object versions in a bucket.
//
// This issues a versions listing with the X-Tigris-Soft-Delete header so the
// server returns soft-deleted versions enriched with their original size, ETag,
// and a SoftDeleted flag — fields the standard S3 SDK does not surface on delete
// markers. Use the returned VersionID with RestoreSoftDeletedObject to recover a
// version or PermanentlyDeleteObject to purge it.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) ListSoftDeletedObjects(ctx context.Context, in *ListSoftDeletedObjectsInput) (*ListSoftDeletedObjectsOutput, error) {
	if in.Bucket == "" {
		return nil, fmt.Errorf("storage: ListSoftDeletedObjects: %w", ErrMissingBucket)
	}

	q := url.Values{}
	if in.Prefix != "" {
		q.Set("prefix", in.Prefix)
	}
	if in.KeyMarker != "" {
		q.Set("key-marker", in.KeyMarker)
	}
	if in.VersionIDMarker != "" {
		q.Set("version-id-marker", in.VersionIDMarker)
	}
	if in.MaxKeys > 0 {
		q.Set("max-keys", strconv.FormatInt(int64(in.MaxKeys), 10))
	}

	rawQuery := "versions"
	if encoded := q.Encode(); encoded != "" {
		rawQuery += "&" + encoded
	}

	reqURL := c.bucketURL(in.Bucket, rawQuery)

	resp, err := c.doSignedRequest(ctx, http.MethodGet, reqURL, map[string]string{
		"X-Tigris-Soft-Delete": "true",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedObjects: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, httpError(resp, "ListSoftDeletedObjects")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedObjects: failed to read response: %w", err)
	}

	var parsed listVersionsResult
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedObjects: failed to parse response: %w", err)
	}

	out := &ListSoftDeletedObjectsOutput{
		IsTruncated:         parsed.IsTruncated,
		NextKeyMarker:       parsed.NextKeyMarker,
		NextVersionIDMarker: parsed.NextVersionIDMarker,
		Objects:             make([]SoftDeletedObject, 0, len(parsed.DeleteMarkers)),
	}
	for _, dm := range parsed.DeleteMarkers {
		out.Objects = append(out.Objects, SoftDeletedObject{
			Key:          dm.Key,
			VersionID:    dm.VersionID,
			Size:         dm.Size,
			ETag:         dm.ETag,
			SoftDeleted:  dm.SoftDeleted,
			LastModified: dm.LastModified,
		})
	}

	return out, nil
}

// RestoreSoftDeletedObject restores a soft-deleted object, undoing a delete
// before its retention window expires.
//
// Pass an empty versionID to restore the most recent soft-deleted version, or a
// specific VersionID from ListSoftDeletedObjects to restore that version.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) RestoreSoftDeletedObject(ctx context.Context, bucket, key, versionID string) error {
	if bucket == "" {
		return fmt.Errorf("storage: RestoreSoftDeletedObject: %w", ErrMissingBucket)
	}
	if key == "" {
		return fmt.Errorf("storage: RestoreSoftDeletedObject: %w", ErrMissingKey)
	}

	headers := map[string]string{
		"X-Tigris-Restore-Type": "soft-delete",
	}
	if versionID != "" {
		headers["X-Tigris-Restore-Version"] = versionID
	}

	reqURL := c.objectURL(bucket, key, "restore")

	resp, err := c.doSignedRequest(ctx, http.MethodPost, reqURL, headers, nil)
	if err != nil {
		return fmt.Errorf("storage: RestoreSoftDeletedObject: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return httpError(resp, "RestoreSoftDeletedObject")
	}

	return nil
}

// RestoreBucket restores a soft-deleted bucket, recovering it and its contents
// before the retention window expires.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) RestoreBucket(ctx context.Context, bucket string) error {
	if bucket == "" {
		return fmt.Errorf("storage: RestoreBucket: %w", ErrMissingBucket)
	}

	reqURL := c.bucketURL(bucket, "restore")

	resp, err := c.doSignedRequest(ctx, http.MethodPost, reqURL, nil, nil)
	if err != nil {
		return fmt.Errorf("storage: RestoreBucket: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return httpError(resp, "RestoreBucket")
	}

	return nil
}

type softDeleteConfig struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days,omitempty"`
}

type patchBucketBody struct {
	SoftDelete softDeleteConfig `json:"soft_delete"`
}

// SetBucketSoftDelete enables or disables soft delete on an existing bucket.
//
// When enabling, pass retentionDays as 0 to use the default 7-day window, or a
// value between 7 and 90 for a custom window. retentionDays is ignored when
// disabling.
//
// See the Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) SetBucketSoftDelete(ctx context.Context, bucket string, enabled bool, retentionDays int) error {
	if bucket == "" {
		return fmt.Errorf("storage: SetBucketSoftDelete: %w", ErrMissingBucket)
	}

	cfg := softDeleteConfig{Enabled: enabled}
	if enabled && retentionDays > 0 {
		cfg.RetentionDays = retentionDays
	}

	body, err := json.Marshal(patchBucketBody{SoftDelete: cfg})
	if err != nil {
		return fmt.Errorf("storage: SetBucketSoftDelete: failed to marshal body: %w", err)
	}

	reqURL := c.bucketURL(bucket, "")

	resp, err := c.doSignedRequest(ctx, http.MethodPatch, reqURL, map[string]string{
		"Content-Type": "application/json",
	}, body)
	if err != nil {
		return fmt.Errorf("storage: SetBucketSoftDelete: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return httpError(resp, "SetBucketSoftDelete")
	}

	return nil
}

// bucketURL builds a path-style URL for a bucket, with an optional raw query.
func (c *Client) bucketURL(bucket, rawQuery string) string {
	return c.baseEndpoint() + "/" + bucket + queryString(rawQuery)
}

// objectURL builds a path-style URL for an object, escaping the key so the
// signed canonical URI matches the request that is sent.
func (c *Client) objectURL(bucket, key, rawQuery string) string {
	u, err := url.Parse(c.baseEndpoint())
	if err != nil {
		// baseEndpoint is derived from a validated client option; fall back to a
		// best-effort string join if it somehow fails to parse.
		return c.baseEndpoint() + "/" + bucket + "/" + key + queryString(rawQuery)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + bucket + "/" + key
	u.RawQuery = rawQuery
	return u.String()
}

func queryString(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	return "?" + rawQuery
}

// httpError reads a bounded amount of an error response body and wraps it.
func httpError(resp *http.Response, op string) error {
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("storage: %s: HTTP %d: %s", op, resp.StatusCode, string(errBody))
}
