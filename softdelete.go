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

// CreateBucketWithSoftDeleteInput is the input for a CreateBucketWithSoftDelete request.
type CreateBucketWithSoftDeleteInput struct {
	// CreateBucketInput carries the standard S3 create-bucket parameters. Required.
	*s3.CreateBucketInput

	// RetentionDays sets the soft delete retention window in days. Use 0 for the
	// default 7-day window, or a value between 7 and 90 for a custom window.
	// Values outside the 7-90 range are rejected by the server.
	RetentionDays int
}

// CreateBucketWithSoftDelete creates a bucket with soft delete enabled. Deleting
// an object or the bucket then moves it into a recoverable soft-deleted state for
// the retention window instead of removing it immediately.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) CreateBucketWithSoftDelete(ctx context.Context, in *CreateBucketWithSoftDeleteInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if in == nil || in.CreateBucketInput == nil {
		return nil, fmt.Errorf("storage: CreateBucketWithSoftDelete: %w", ErrMissingBucket)
	}

	// Only 0 means "use the default window". Any other value, negative included,
	// goes to the server so an out-of-range window is reported as an error
	// instead of silently becoming the 7-day default.
	if in.RetentionDays != 0 {
		optFns = append(optFns, tigrisheaders.WithSoftDelete(in.RetentionDays))
	} else {
		optFns = append(optFns, tigrisheaders.WithSoftDelete())
	}

	return c.Client.CreateBucket(ctx, in.CreateBucketInput, optFns...)
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
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) ForceDeleteBucket(ctx context.Context, in *s3.DeleteBucketInput, opts ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	if in == nil || in.Bucket == nil || *in.Bucket == "" {
		return nil, fmt.Errorf("storage: ForceDeleteBucket: %w", ErrMissingBucket)
	}

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
// version, the opposite of this method's intent. The value is the deletion
// timestamp in nanoseconds since the Unix epoch. Tigris rejects a value that is
// zero or negative, so pass the VersionID from ListSoftDeletedObjects unchanged.
//
// This is a dangerous operation. Do not use this unless you are aware of
// the consequences of your actions. Support will not be able to help you
// recover any buckets or objects deleted in this way.
//
// See Tigris documentation[1] for more information.
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

	// X-Tigris-Soft-Delete carries two unrelated meanings depending on the
	// route. On CreateBucket (tigrisheaders.WithSoftDelete) it enables retention.
	// On DeleteObject, as here, it means "act on the soft-delete state", which
	// purges the named version instead of creating a new tombstone.
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
// with the Tigris-specific SoftDeleted flag that the AWS SDK does not model.
//
// Tigris returns soft-delete tombstones as Version elements, not DeleteMarker
// elements, because the S3 DeleteMarker schema has nowhere to carry Size or
// ETag. Regular S3 delete markers are still returned as DeleteMarker elements
// and are deliberately not decoded here: they are not soft-deleted.
type listVersionsResult struct {
	XMLName             xml.Name       `xml:"ListVersionsResult"`
	IsTruncated         bool           `xml:"IsTruncated"`
	NextKeyMarker       string         `xml:"NextKeyMarker"`
	NextVersionIDMarker string         `xml:"NextVersionIdMarker"`
	Versions            []versionEntry `xml:"Version"`
}

type versionEntry struct {
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
// This method uses a Tigris endpoint that the S3 SDK cannot express, so it
// sends a signed request directly and does not accept s3.Options functions.
// Options such as tigrisheaders.WithHeader have no effect here.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) ListSoftDeletedObjects(ctx context.Context, in *ListSoftDeletedObjectsInput) (*ListSoftDeletedObjectsOutput, error) {
	if in == nil || in.Bucket == "" {
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
		Objects:             make([]SoftDeletedObject, 0, len(parsed.Versions)),
	}
	for _, v := range parsed.Versions {
		// A versioned bucket returns its live versions in the same listing.
		// Only the tombstones carry SoftDeleted.
		if !v.SoftDeleted {
			continue
		}
		out.Objects = append(out.Objects, SoftDeletedObject(v))
	}

	return out, nil
}

// RestoreSoftDeletedObjectInput is the input for a RestoreSoftDeletedObject request.
type RestoreSoftDeletedObjectInput struct {
	// Bucket is the name of the bucket containing the object. Required.
	Bucket string
	// Key is the object key to restore. Required.
	Key string
	// VersionID restores a specific soft-deleted version, as returned by
	// ListSoftDeletedObjects. The value is the deletion timestamp in nanoseconds
	// since the Unix epoch. Optional; empty restores the most recent
	// soft-deleted version.
	VersionID string
}

// RestoreSoftDeletedObjectOutput is the response from a RestoreSoftDeletedObject request.
type RestoreSoftDeletedObjectOutput struct{}

// RestoreSoftDeletedObject restores a soft-deleted object, undoing a delete
// before its retention window expires.
//
// This method uses a Tigris endpoint that the S3 SDK cannot express, so it
// sends a signed request directly and does not accept s3.Options functions.
// Options such as tigrisheaders.WithHeader have no effect here.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) RestoreSoftDeletedObject(ctx context.Context, in *RestoreSoftDeletedObjectInput) (*RestoreSoftDeletedObjectOutput, error) {
	if in == nil || in.Bucket == "" {
		return nil, fmt.Errorf("storage: RestoreSoftDeletedObject: %w", ErrMissingBucket)
	}
	if in.Key == "" {
		return nil, fmt.Errorf("storage: RestoreSoftDeletedObject: %w", ErrMissingKey)
	}

	headers := map[string]string{
		"X-Tigris-Restore-Type": "soft-delete",
	}
	if in.VersionID != "" {
		headers["X-Tigris-Restore-Version"] = in.VersionID
	}

	reqURL := c.objectURL(in.Bucket, in.Key, "restore")

	resp, err := c.doSignedRequest(ctx, http.MethodPost, reqURL, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: RestoreSoftDeletedObject: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, httpError(resp, "RestoreSoftDeletedObject")
	}

	return &RestoreSoftDeletedObjectOutput{}, nil
}

// RestoreBucketInput is the input for a RestoreBucket request.
type RestoreBucketInput struct {
	// Bucket is the name of the soft-deleted bucket to restore. Required.
	Bucket string
}

// RestoreBucketOutput is the response from a RestoreBucket request.
type RestoreBucketOutput struct{}

// RestoreBucket restores a soft-deleted bucket, recovering it and its contents
// before the retention window expires.
//
// This method uses a Tigris endpoint that the S3 SDK cannot express, so it
// sends a signed request directly and does not accept s3.Options functions.
// Options such as tigrisheaders.WithHeader have no effect here.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) RestoreBucket(ctx context.Context, in *RestoreBucketInput) (*RestoreBucketOutput, error) {
	if in == nil || in.Bucket == "" {
		return nil, fmt.Errorf("storage: RestoreBucket: %w", ErrMissingBucket)
	}

	reqURL := c.bucketURL(in.Bucket, "restore")

	resp, err := c.doSignedRequest(ctx, http.MethodPost, reqURL, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: RestoreBucket: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, httpError(resp, "RestoreBucket")
	}

	return &RestoreBucketOutput{}, nil
}

// SoftDeletedBucket describes a single soft-deleted bucket returned by
// ListSoftDeletedBuckets.
type SoftDeletedBucket struct {
	// Name is the bucket name. It stays reserved while the bucket is
	// soft-deleted.
	Name string
	// InitialCreatedDate is the time the bucket was first created.
	InitialCreatedDate time.Time
	// CreationDate is the time the bucket was most recently created.
	CreationDate time.Time
	// RetentionDays is the soft delete retention window in days. The bucket is
	// permanently removed once the window expires.
	RetentionDays int
}

// ListSoftDeletedBucketsInput is the input for a ListSoftDeletedBuckets request.
type ListSoftDeletedBucketsInput struct {
	// ContinuationToken continues a truncated listing from NextContinuationToken. Optional.
	ContinuationToken string
	// MaxBuckets limits the number of buckets returned. Optional; 0 uses the server default.
	MaxBuckets int32
}

// ListSoftDeletedBucketsOutput is the response from a ListSoftDeletedBuckets request.
type ListSoftDeletedBucketsOutput struct {
	// Buckets is the list of soft-deleted buckets.
	Buckets []SoftDeletedBucket
	// IsTruncated reports whether more results are available beyond this page.
	IsTruncated bool
	// NextContinuationToken is the token to pass as ContinuationToken for the next page.
	NextContinuationToken string
}

// listAllMyBucketsResult mirrors the S3 ListAllMyBucketsResult XML response,
// augmented with the Tigris-specific SoftDeleteInfo fields that the AWS SDK
// does not model.
//
// ContinuationToken is the only pagination field Tigris returns here. There is
// no IsTruncated element, so more pages are available exactly when the token is
// not empty.
type listAllMyBucketsResult struct {
	XMLName           xml.Name             `xml:"ListAllMyBucketsResult"`
	ContinuationToken string               `xml:"ContinuationToken"`
	Buckets           []bucketListingEntry `xml:"Buckets>Bucket"`
}

type bucketListingEntry struct {
	Name               string    `xml:"Name"`
	InitialCreatedDate time.Time `xml:"InitialCreatedDate"`
	CreationDate       time.Time `xml:"CreationDate"`
	RetentionDays      int       `xml:"SoftDeleteInfo>RetentionDays"`
}

// ListSoftDeletedBuckets lists the soft-deleted buckets in the account.
//
// This issues a bucket listing with the OnlyDeleted query parameter so the
// server returns only soft-deleted buckets, each enriched with its retention
// window — a field the standard S3 SDK does not surface. Use the returned Name
// with RestoreBucket to recover a bucket before its window expires. A nil
// input lists the first page with the server default page size.
//
// This method uses a Tigris endpoint that the S3 SDK cannot express, so it
// sends a signed request directly and does not accept s3.Options functions.
// Options such as tigrisheaders.WithHeader have no effect here.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) ListSoftDeletedBuckets(ctx context.Context, in *ListSoftDeletedBucketsInput) (*ListSoftDeletedBucketsOutput, error) {
	// OnlyDeleted is a Tigris filter and is read PascalCase. The pagination
	// parameters are the standard S3 ones and are read kebab-case. The names
	// are case-sensitive server-side, so this asymmetry is deliberate.
	q := url.Values{}
	q.Set("OnlyDeleted", "true")
	if in != nil {
		if in.ContinuationToken != "" {
			q.Set("continuation-token", in.ContinuationToken)
		}
		if in.MaxBuckets > 0 {
			q.Set("max-buckets", strconv.FormatInt(int64(in.MaxBuckets), 10))
		}
	}

	reqURL := c.baseEndpoint() + "/" + queryString(q.Encode())

	resp, err := c.doSignedRequest(ctx, http.MethodGet, reqURL, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedBuckets: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, httpError(resp, "ListSoftDeletedBuckets")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedBuckets: failed to read response: %w", err)
	}

	var parsed listAllMyBucketsResult
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("storage: ListSoftDeletedBuckets: failed to parse response: %w", err)
	}

	out := &ListSoftDeletedBucketsOutput{
		IsTruncated:           parsed.ContinuationToken != "",
		NextContinuationToken: parsed.ContinuationToken,
		Buckets:               make([]SoftDeletedBucket, 0, len(parsed.Buckets)),
	}
	for _, b := range parsed.Buckets {
		out.Buckets = append(out.Buckets, SoftDeletedBucket(b))
	}

	return out, nil
}

type softDeleteConfig struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retention_days,omitempty"`
}

type patchBucketBody struct {
	SoftDelete softDeleteConfig `json:"soft_delete"`
}

// SetBucketSoftDeleteInput is the input for a SetBucketSoftDelete request.
type SetBucketSoftDeleteInput struct {
	// Bucket is the name of the bucket to configure. Required.
	Bucket string
	// Enabled turns soft delete on or off for the bucket.
	Enabled bool
	// RetentionDays sets the soft delete retention window in days when enabling.
	// Use 0 for the default 7-day window, or a value between 7 and 90 for a custom
	// window. Ignored when disabling.
	RetentionDays int
}

// SetBucketSoftDeleteOutput is the response from a SetBucketSoftDelete request.
type SetBucketSoftDeleteOutput struct{}

// SetBucketSoftDelete enables or disables soft delete on an existing bucket.
//
// This method uses a Tigris endpoint that the S3 SDK cannot express, so it
// sends a signed request directly and does not accept s3.Options functions.
// Options such as tigrisheaders.WithHeader have no effect here.
//
// See Tigris documentation[1] for more information.
//
// [1]: https://www.tigrisdata.com/docs/buckets/soft-delete/
func (c *Client) SetBucketSoftDelete(ctx context.Context, in *SetBucketSoftDeleteInput) (*SetBucketSoftDeleteOutput, error) {
	if in == nil || in.Bucket == "" {
		return nil, fmt.Errorf("storage: SetBucketSoftDelete: %w", ErrMissingBucket)
	}

	// As in CreateBucketWithSoftDelete, only 0 means "use the default window".
	cfg := softDeleteConfig{Enabled: in.Enabled}
	if in.Enabled && in.RetentionDays != 0 {
		cfg.RetentionDays = in.RetentionDays
	}

	body, err := json.Marshal(patchBucketBody{SoftDelete: cfg})
	if err != nil {
		return nil, fmt.Errorf("storage: SetBucketSoftDelete: failed to marshal body: %w", err)
	}

	reqURL := c.bucketURL(in.Bucket, "")

	resp, err := c.doSignedRequest(ctx, http.MethodPatch, reqURL, map[string]string{
		"Content-Type": "application/json",
	}, body)
	if err != nil {
		return nil, fmt.Errorf("storage: SetBucketSoftDelete: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, httpError(resp, "SetBucketSoftDelete")
	}

	return &SetBucketSoftDeleteOutput{}, nil
}

// bucketURL builds a path-style URL for a bucket, with an optional raw query.
func (c *Client) bucketURL(bucket, rawQuery string) string {
	return c.baseEndpoint() + "/" + bucket + queryString(rawQuery)
}

// objectURL builds a path-style URL for an object.
//
// The key is assigned to url.URL.Path in its decoded form; url.URL.String
// percent-encodes it on serialization (including %, ?, and #) while preserving
// "/" as path separators, so arbitrary keys round-trip correctly.
// url.PathEscape is deliberately not used: it would encode "/" separators and
// corrupt multi-segment keys.
//
// The path is therefore already encoded once by the time it is signed, which is
// why doSignedRequest sets DisableURIPathEscaping. Without that option the
// signer encodes it again and the canonical URI stops matching the request.
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
