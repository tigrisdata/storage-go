package simplestorage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	tmtypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	storage "github.com/tigrisdata/storage-go"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

// AccessType controls whether an object or bucket is publicly readable.
type AccessType string

const (
	// AccessPrivate restricts access to authenticated callers (S3 canned ACL "private").
	AccessPrivate AccessType = "private"
	// AccessPublic makes the object or bucket world-readable (S3 canned ACL "public-read").
	AccessPublic AccessType = "public"
)

// ErrNoBucketName is returned when no bucket name is provided via the
// TIGRIS_STORAGE_BUCKET environment variable or the WithBucket option.
var ErrNoBucketName = errors.New("bucket name not set: provide the TIGRIS_STORAGE_BUCKET environment variable or use WithBucket option")

// Client is a high-level client for Tigris that simplifies common interactions
// to very high level calls.
type Client struct {
	cli     *storage.Client
	options Options
}

// ClientOption is a function option that allows callers to override settings in
// calls to Tigris via Client.
type ClientOption func(*ClientOptions)

// OverrideBucket overrides the bucket used for Tigris calls.
func OverrideBucket(bucket string) ClientOption {
	return func(co *ClientOptions) {
		co.BucketName = bucket
	}
}

// WithS3Options sets S3 options for individual Tigris calls.
func WithS3Options(opts ...func(*s3.Options)) ClientOption {
	return func(co *ClientOptions) {
		co.S3Options = append(co.S3Options, opts...)
	}
}

// WithStartAfter sets the StartAfter setting in List calls. Use this if you need
// pagination in your List calls.
func WithStartAfter(startAfter string) ClientOption {
	return func(co *ClientOptions) {
		co.StartAfter = aws.String(startAfter)
	}
}

// WithMaxKeys sets the maximum number of keys in List calls. Use this along with
// WithStartAfter for pagination in your List calls.
func WithMaxKeys(maxKeys int32) ClientOption {
	return func(co *ClientOptions) {
		co.MaxKeys = &maxKeys
	}
}

// WithDelimiter sets the delimiter character for grouping keys in List calls.
// Commonly set to "/" to emulate directory-like grouping.
func WithDelimiter(delimiter string) ClientOption {
	return func(co *ClientOptions) {
		co.Delimiter = aws.String(delimiter)
	}
}

// WithPaginationToken sets the continuation token for paginated List calls.
// Use the value of ListResult.PaginationToken from a previous page.
func WithPaginationToken(token string) ClientOption {
	return func(co *ClientOptions) {
		co.PaginationToken = aws.String(token)
	}
}

// WithQuerySnapshotVersion specifies a snapshot version to query for Get, Head, or List operations.
// Use this to read from a specific bucket snapshot.
func WithQuerySnapshotVersion(version string) ClientOption {
	return func(co *ClientOptions) {
		co.SnapshotVersion = aws.String(version)
		co.S3Options = append(co.S3Options, tigrisheaders.WithSnapshotVersion(version))
	}
}

// WithResponseContentType overrides the Content-Type header in Get responses.
func WithResponseContentType(contentType string) ClientOption {
	return func(co *ClientOptions) {
		co.ResponseContentType = aws.String(contentType)
	}
}

// WithResponseContentDisposition overrides the Content-Disposition header in Get responses.
func WithResponseContentDisposition(disposition string) ClientOption {
	return func(co *ClientOptions) {
		co.ResponseContentDisposition = aws.String(disposition)
	}
}

// WithResponseCacheControl overrides the Cache-Control header in Get responses.
func WithResponseCacheControl(cacheControl string) ClientOption {
	return func(co *ClientOptions) {
		co.ResponseCacheControl = aws.String(cacheControl)
	}
}

// WithRandomSuffix adds a random suffix to the object key for uniqueness in Put operations.
func WithRandomSuffix() ClientOption {
	return func(co *ClientOptions) {
		co.RandomSuffix = true
	}
}

// WithAllowOverwrite controls whether overwrites are permitted in Put operations.
// When set to false, the operation will fail if the object already exists.
func WithAllowOverwrite(allow bool) ClientOption {
	return func(co *ClientOptions) {
		co.AllowOverwrite = aws.Bool(allow)
	}
}

// WithContentDisposition sets the Content-Disposition header for Put operations.
func WithContentDisposition(disposition string) ClientOption {
	return func(co *ClientOptions) {
		co.ContentDisposition = aws.String(disposition)
	}
}

// WithMultipartUpload enables multipart upload for files above the specified size threshold in bytes.
func WithMultipartUpload(threshold int64) ClientOption {
	return func(co *ClientOptions) {
		co.MultipartThreshold = aws.Int64(threshold)
	}
}

// WithAccessType sets the canned ACL for Put operations. Use AccessPublic to
// make the uploaded object world-readable or AccessPrivate for authenticated-only
// access. If unset, the bucket's default ACL applies.
func WithAccessType(access AccessType) ClientOption {
	return func(co *ClientOptions) {
		co.AccessType = access
	}
}

// UploadProgress tracks upload progress.
type UploadProgress struct {
	Loaded     int64   // Bytes uploaded
	Total      int64   // Total bytes
	Percentage float64 // Percentage complete
}

// WithUploadProgress sets a callback function to track upload progress in Put operations.
func WithUploadProgress(callback func(UploadProgress)) ClientOption {
	return func(co *ClientOptions) {
		co.UploadProgressCallback = callback
	}
}

// WithPresignedExpiresIn sets the expiration time for presigned URLs (in seconds).
func WithPresignedExpiresIn(expiresIn int) ClientOption {
	return func(co *ClientOptions) {
		co.PresignedExpiresIn = aws.Int(expiresIn)
	}
}

// WithPresignedContentType sets the content type for presigned PUT URLs.
func WithPresignedContentType(contentType string) ClientOption {
	return func(co *ClientOptions) {
		co.PresignedContentType = aws.String(contentType)
	}
}

// PresignOperation specifies the HTTP operation that a presigned URL authorizes.
type PresignOperation string

const (
	// PresignOpGet authorizes a GET (download) on the object.
	PresignOpGet PresignOperation = "get"
	// PresignOpPut authorizes a PUT (upload) on the object.
	PresignOpPut PresignOperation = "put"
)

// WithPresignedOperation selects the HTTP operation for GetPresignedUrl.
// Defaults to PresignOpGet when unset.
func WithPresignedOperation(op PresignOperation) ClientOption {
	return func(co *ClientOptions) {
		co.PresignedOperation = op
	}
}

// WithPrefix sets the prefix filter for List operations.
func WithPrefix(prefix string) ClientOption {
	return func(co *ClientOptions) {
		co.Prefix = aws.String(prefix)
	}
}

// ClientOptions is the collection of options that are set for individual Tigris
// calls.
type ClientOptions struct {
	BucketName string
	S3Options  []func(*s3.Options)

	// List options
	Prefix          *string
	StartAfter      *string
	MaxKeys         *int32
	Delimiter       *string
	PaginationToken *string

	// Snapshot version for Get, Head, List operations
	SnapshotVersion *string

	// Response override options for Get operations
	ResponseContentType        *string
	ResponseContentDisposition *string
	ResponseCacheControl       *string

	// Put options
	RandomSuffix           bool
	AllowOverwrite         *bool
	ContentDisposition     *string
	MultipartThreshold     *int64
	UploadProgressCallback func(UploadProgress)
	AccessType             AccessType

	// Presigned URL options
	PresignedExpiresIn   *int
	PresignedContentType *string
	PresignedOperation   PresignOperation
}

// HeadResponse contains metadata about an object returned by Head.
type HeadResponse struct {
	Path               string    // Object key
	Size               int64     // Size in bytes
	Modified           time.Time // Last modified time
	ContentType        string    // MIME type
	ContentDisposition string    // Content-Disposition header
	URL                string    // Presigned URL for the object
}

// PutResponse contains the result of a Put operation.
type PutResponse struct {
	Path               string    // Object key
	Size               int64     // Size in bytes
	Modified           time.Time // Last modified time
	ContentType        string    // MIME type
	ContentDisposition string    // Content-Disposition header
	URL                string    // Presigned URL for the object
}

// ListResult contains the results of a List operation.
type ListResult struct {
	Items           []Object // List of objects
	CommonPrefixes  []string // Common prefixes grouped by delimiter (populated when WithDelimiter is set)
	PaginationToken string   // Token for next page
	HasMore         bool     // Whether more results exist
}

// GetPresignedUrlResult contains the result of GetPresignedUrl.
type GetPresignedUrlResult struct {
	URL       string // Presigned URL
	ExpiresIn int    // Expiration time in seconds
	Method    string // HTTP method ('get' or 'put')
}

// defaults populates client options from the global Options.
func (ClientOptions) defaults(o Options) ClientOptions {
	return ClientOptions{
		BucketName: o.BucketName,
	}
}

// New creates a new Client based on the options provided and defaults loaded from the environment.
//
// By default New reads the following environment variables for setting its defaults:
//
// * `TIGRIS_STORAGE_BUCKET`: the name of the bucket for all Tigris operations. If this is not set in the environment or via the WithBucket, New() will return an error containing ErrNoBucketName.
// * `TIGRIS_STORAGE_ACCESS_KEY_ID`: The access key ID of the Tigris authentication keypair. If this is not set in the environment or via WithAccessKeypair, New() will load configuration via the AWS configuration resolution method.
// * `TIGRIS_STORAGE_SECRET_ACCESS_KEY`: The secret access key of the Tigris authentication keypair. If this is not set in the environment or via WithAccessKeypair, New() will load configuration via the AWS configuration resolution method.
//
// The returned Client will default to having its operations performed on the specified bucket. If
// individual calls need to operate against arbitrary buckets, override it with OverrideBucket.
func New(ctx context.Context, options ...Option) (*Client, error) {
	o := new(Options).defaults()

	for _, doer := range options {
		doer(&o)
	}

	var errs []error
	if o.BucketName == "" {
		errs = append(errs, ErrNoBucketName)
	}

	if len(errs) != 0 {
		return nil, fmt.Errorf("simplestorage: can't create client: %w", errors.Join(errs...))
	}

	var storageOpts []storage.Option

	if o.BaseEndpoint != storage.GlobalEndpoint {
		storageOpts = append(storageOpts, storage.WithEndpoint(o.BaseEndpoint))
	}

	storageOpts = append(storageOpts, storage.WithRegion(o.Region))
	storageOpts = append(storageOpts, storage.WithPathStyle(o.UsePathStyle))

	if o.AccessKeyID != "" && o.SecretAccessKey != "" {
		storageOpts = append(storageOpts, storage.WithAccessKeypair(o.AccessKeyID, o.SecretAccessKey))
	}

	cli, err := storage.New(ctx, storageOpts...)
	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't create storage client: %w", err)
	}

	return &Client{
		cli:     cli,
		options: o,
	}, nil
}

// Object contains metadata about an individual object read from or put into Tigris.
//
// Some calls may not populate all fields. Ensure that the values are valid before
// consuming them.
type Object struct {
	Bucket       string        // Bucket the object is in
	Key          string        // Key for the object
	ContentType  string        // MIME type for the object or application/octet-stream
	Etag         string        // Entity tag for the object (usually a checksum)
	Version      string        // Version tag for the object
	Size         int64         // Size of the object in bytes or 0 if unknown
	LastModified time.Time     // Creation date of the object
	Body         io.ReadCloser // Body of the object so it can be read, don't forget to close it.
}

// Get fetches the contents of an object and its metadata from Tigris.
func (c *Client) Get(ctx context.Context, key string, opts ...ClientOption) (*Object, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket:                     aws.String(o.BucketName),
			Key:                        aws.String(key),
			ResponseContentType:        o.ResponseContentType,
			ResponseContentDisposition: o.ResponseContentDisposition,
			ResponseCacheControl:       o.ResponseCacheControl,
		},
		o.S3Options...,
	)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't get %s/%s: %v", o.BucketName, key, err)
	}

	return &Object{
		Bucket:       o.BucketName,
		Key:          key,
		ContentType:  lower(resp.ContentType, "application/octet-stream"),
		Etag:         lower(resp.ETag, ""),
		Size:         lower(resp.ContentLength, 0),
		Version:      lower(resp.VersionId, ""),
		LastModified: lower(resp.LastModified, time.Time{}),
		Body:         resp.Body,
	}, nil
}

// Head fetches metadata about an object without downloading its body.
// Returns HeadResponse with metadata and a presigned URL for accessing the object.
func (c *Client) Head(ctx context.Context, key string, opts ...ClientOption) (*HeadResponse, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: aws.String(o.BucketName),
			Key:    aws.String(key),
		},
		o.S3Options...,
	)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't head %s/%s: %v", o.BucketName, key, err)
	}

	// Create presigner for generating presigned URL
	presignClient := s3.NewPresignClient(c.cli.S3(), func(po *s3.PresignOptions) {
		po.ClientOptions = o.S3Options
	})

	// Generate presigned GET URL for the object
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't presign get for %s/%s: %v", o.BucketName, key, err)
	}

	return &HeadResponse{
		Path:               key,
		Size:               lower(resp.ContentLength, 0),
		Modified:           lower(resp.LastModified, time.Time{}),
		ContentType:        lower(resp.ContentType, "application/octet-stream"),
		ContentDisposition: lower(resp.ContentDisposition, ""),
		URL:                presignResult.URL,
	}, nil
}

// Put puts the contents of an object into Tigris.
// Returns PutResponse with metadata and a presigned URL for accessing the uploaded object.
func (c *Client) Put(ctx context.Context, obj *Object, opts ...ClientOption) (*PutResponse, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	// Handle random suffix
	key := obj.Key
	if o.RandomSuffix {
		suffix := generateRandomSuffix(12)
		key = fmt.Sprintf("%s-%s", key, suffix)
	}

	// Disallow overwrites server-side using If-Match: "" so the check is atomic.
	if o.AllowOverwrite != nil && !*o.AllowOverwrite {
		o.S3Options = append(o.S3Options, tigrisheaders.WithCreateObjectIfNotExists())
	}

	body := obj.Body
	if o.UploadProgressCallback != nil && body != nil {
		body = &progressReader{
			reader:   body,
			total:    obj.Size,
			callback: o.UploadProgressCallback,
		}
	}

	useMultipart := o.MultipartThreshold != nil && obj.Size > *o.MultipartThreshold

	putInput := &s3.PutObjectInput{
		Bucket:             aws.String(o.BucketName),
		Key:                aws.String(key),
		Body:               body,
		ContentType:        raise(obj.ContentType),
		ContentDisposition: o.ContentDisposition,
		ACL:                objectACL(o.AccessType),
	}
	// manager.Uploader reads the body in chunks, so don't force a ContentLength
	// on multipart uploads.
	if !useMultipart {
		putInput.ContentLength = raise(obj.Size)
	}

	var err error
	if useMultipart {
		tm := transfermanager.New(c.cli.S3())
		_, err = tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Bucket:             putInput.Bucket,
			Key:                putInput.Key,
			Body:               putInput.Body,
			ContentType:        putInput.ContentType,
			ContentDisposition: putInput.ContentDisposition,
			ACL:                tmObjectACL(o.AccessType),
		})
	} else {
		_, err = c.cli.PutObject(ctx, putInput, o.S3Options...)
	}

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't put %s/%s: %v", o.BucketName, key, err)
	}

	// Create presigner for generating presigned URL
	presignClient := s3.NewPresignClient(c.cli.S3())

	// Generate presigned GET URL for the uploaded object
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't presign get for %s/%s: %v", o.BucketName, key, err)
	}

	return &PutResponse{
		Path:               key,
		Size:               obj.Size,
		Modified:           time.Now(),
		ContentType:        obj.ContentType,
		ContentDisposition: lower(o.ContentDisposition, ""),
		URL:                presignResult.URL,
	}, nil
}

// generateRandomSuffix generates a random hexadecimal string of the specified length.
func generateRandomSuffix(length int) string {
	b := make([]byte, length/2)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// progressReader wraps an io.Reader to invoke a callback with cumulative
// progress after each Read. Retries may cause the callback to observe bytes
// more than once; callers should treat the Loaded counter as monotonic within
// a single attempt, not across the whole operation.
type progressReader struct {
	reader   io.Reader
	total    int64
	loaded   int64
	callback func(UploadProgress)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.loaded += int64(n)
		var pct float64
		if pr.total > 0 {
			pct = float64(pr.loaded) / float64(pr.total) * 100
		}
		pr.callback(UploadProgress{Loaded: pr.loaded, Total: pr.total, Percentage: pct})
	}
	return n, err
}

// Close forwards to the underlying reader's Close method when present.
func (pr *progressReader) Close() error {
	if rc, ok := pr.reader.(io.Closer); ok {
		return rc.Close()
	}
	return nil
}

// objectACL maps an AccessType to an S3 canned ACL for object operations.
// Returns an empty ACL when access is unset so the bucket default applies.
func objectACL(a AccessType) s3types.ObjectCannedACL {
	switch a {
	case AccessPublic:
		return s3types.ObjectCannedACLPublicRead
	case AccessPrivate:
		return s3types.ObjectCannedACLPrivate
	default:
		return ""
	}
}

// tmObjectACL mirrors objectACL for the transfermanager package's ACL type.
func tmObjectACL(a AccessType) tmtypes.ObjectCannedACL {
	switch a {
	case AccessPublic:
		return tmtypes.ObjectCannedACLPublicRead
	case AccessPrivate:
		return tmtypes.ObjectCannedACLPrivate
	default:
		return ""
	}
}

// Delete removes an object from Tigris.
func (c *Client) Delete(ctx context.Context, key string, opts ...ClientOption) error {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	if _, err := c.cli.DeleteObject(
		ctx,
		&s3.DeleteObjectInput{
			Bucket: aws.String(o.BucketName),
			Key:    aws.String(key),
		},
		o.S3Options...,
	); err != nil {
		return fmt.Errorf("simplestorage: can't delete %s/%s: %v", o.BucketName, key, err)
	}

	return nil
}

// List returns a list of objects matching a key prefix.
// Returns ListResult with Items, PaginationToken, and HasMore.
func (c *Client) List(ctx context.Context, prefix string, opts ...ClientOption) (*ListResult, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	// Use prefix from option if provided
	listPrefix := aws.String(prefix)
	if o.Prefix != nil {
		listPrefix = o.Prefix
	}

	resp, err := c.cli.ListObjectsV2(
		ctx,
		&s3.ListObjectsV2Input{
			Bucket:            aws.String(o.BucketName),
			Prefix:            listPrefix,
			Delimiter:         o.Delimiter,
			MaxKeys:           o.MaxKeys,
			StartAfter:        o.StartAfter,
			ContinuationToken: o.PaginationToken,
		},
		o.S3Options...,
	)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't list %s/%s: %v", o.BucketName, prefix, err)
	}

	items := make([]Object, 0, len(resp.Contents))
	for _, obj := range resp.Contents {
		items = append(items, Object{
			Bucket:       o.BucketName,
			Key:          *obj.Key,
			Etag:         lower(obj.ETag, ""),
			Size:         lower(obj.Size, 0),
			LastModified: lower(obj.LastModified, time.Time{}),
		})
	}

	prefixes := make([]string, 0, len(resp.CommonPrefixes))
	for _, p := range resp.CommonPrefixes {
		if p.Prefix != nil {
			prefixes = append(prefixes, *p.Prefix)
		}
	}

	return &ListResult{
		Items:           items,
		CommonPrefixes:  prefixes,
		PaginationToken: lower(resp.NextContinuationToken, ""),
		HasMore:         resp.IsTruncated != nil && *resp.IsTruncated,
	}, nil
}

// GetPresignedUrl generates a presigned URL authorizing a single operation
// (GET or PUT) on an object.
//
// The operation defaults to PresignOpGet. Use WithPresignedOperation to request
// a PUT URL, WithPresignedExpiresIn to override the one-hour default expiration,
// and WithPresignedContentType to bind a Content-Type requirement onto a PUT URL.
func (c *Client) GetPresignedUrl(ctx context.Context, key string, opts ...ClientOption) (*GetPresignedUrlResult, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	expiresIn := 3600 * time.Second
	if o.PresignedExpiresIn != nil {
		expiresIn = time.Duration(*o.PresignedExpiresIn) * time.Second
	}

	op := o.PresignedOperation
	if op == "" {
		op = PresignOpGet
	}

	presignClient := s3.NewPresignClient(c.cli.S3(), s3.WithPresignExpires(expiresIn))

	var presignedURL string
	switch op {
	case PresignOpGet:
		res, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(o.BucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, fmt.Errorf("simplestorage: can't presign get for %s/%s: %v", o.BucketName, key, err)
		}
		presignedURL = res.URL
	case PresignOpPut:
		res, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(o.BucketName),
			Key:         aws.String(key),
			ContentType: o.PresignedContentType,
		})
		if err != nil {
			return nil, fmt.Errorf("simplestorage: can't presign put for %s/%s: %v", o.BucketName, key, err)
		}
		presignedURL = res.URL
	default:
		return nil, fmt.Errorf("simplestorage: unsupported presign operation %q (use PresignOpGet or PresignOpPut)", op)
	}

	return &GetPresignedUrlResult{
		URL:       presignedURL,
		ExpiresIn: int(expiresIn.Seconds()),
		Method:    string(op),
	}, nil
}

// lower lowers the "pointer level" of the value by returning the value pointed
// to by p, or defaultVal if p is nil.
func lower[T any](p *T, defaultVal T) T {
	if p != nil {
		return *p
	}
	return defaultVal
}

// raise raises the "pointer level" of the value by returning a pointer to v,
// or nil if v is the zero value for type T.
func raise[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
