package simplestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	storage "github.com/tigrisdata/storage-go"
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

// WithDelimiter sets a delimiter for grouping keys in List calls.
func WithDelimiter(delimiter string) ClientOption {
	return func(co *ClientOptions) {
		co.Delimiter = aws.String(delimiter)
	}
}

// WithPrefix sets the prefix to filter keys in List calls.
func WithPrefix(prefix string) ClientOption {
	return func(co *ClientOptions) {
		co.Prefix = aws.String(prefix)
	}
}

// WithPaginationToken sets the pagination token to continue listing objects.
func WithPaginationToken(token string) ClientOption {
	return func(co *ClientOptions) {
		co.PaginationToken = aws.String(token)
	}
}

// WithContentType sets the Content-Type header for presigned PUT URLs.
func WithContentType(contentType string) ClientOption {
	return func(co *ClientOptions) {
		co.ContentType = aws.String(contentType)
	}
}

// WithContentDisposition sets the Content-Disposition header for presigned PUT URLs.
func WithContentDisposition(disposition string) ClientOption {
	return func(co *ClientOptions) {
		co.ContentDisposition = aws.String(disposition)
	}
}

// ClientOptions is the collection of options that are set for individual Tigris
// calls.
type ClientOptions struct {
	BucketName string
	S3Options  []func(*s3.Options)

	// List options
	StartAfter      *string
	MaxKeys         *int32
	Delimiter       *string
	Prefix          *string
	PaginationToken *string

	// Presign options
	ContentType        *string
	ContentDisposition *string
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
	Bucket             string            // Bucket the object is in
	Key                string            // Key for the object
	ContentType        string            // MIME type for the object or application/octet-stream
	ContentDisposition string            // Content disposition of the object (inline or attachment)
	Etag               string            // Entity tag for the object (usually a checksum)
	Version            string            // Version tag for the object
	Size               int64             // Size of the object in bytes or 0 if unknown
	LastModified       time.Time         // Creation date of the object
	Metadata           map[string]string // Custom metadata headers
	URL                string            // Public or presigned URL for the object
	Body               io.ReadCloser     // Body of the object so it can be read, don't forget to close it.
}

// ListResult contains the result of a List operation, including pagination information.
type ListResult struct {
	Items     []Object // List of objects
	NextToken string   // Pagination token for the next page
	HasMore   bool     // Whether there are more objects to list
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
			Bucket: aws.String(o.BucketName),
			Key:    aws.String(key),
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
		Metadata:     resp.Metadata,
		Body:         resp.Body,
	}, nil
}

// Head retrieves metadata for an object without downloading its content.
func (c *Client) Head(ctx context.Context, key string, opts ...ClientOption) (*Object, error) {
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

	return &Object{
		Bucket:             o.BucketName,
		Key:                key,
		ContentType:        lower(resp.ContentType, "application/octet-stream"),
		ContentDisposition: lower(resp.ContentDisposition, ""),
		Etag:               lower(resp.ETag, ""),
		Size:               lower(resp.ContentLength, 0),
		Version:            lower(resp.VersionId, ""),
		LastModified:       lower(resp.LastModified, time.Time{}),
		Metadata:           resp.Metadata,
	}, nil
}

// Put puts the contents of an object into Tigris.
func (c *Client) Put(ctx context.Context, obj *Object, opts ...ClientOption) (*Object, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:        aws.String(o.BucketName),
			Key:           aws.String(obj.Key),
			Body:          obj.Body,
			ContentType:   raise(obj.ContentType),
			ContentLength: raise(obj.Size),
		},
		o.S3Options...,
	)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't put %s/%s: %v", o.BucketName, obj.Key, err)
	}

	obj.Bucket = o.BucketName
	obj.Etag = lower(resp.ETag, "")
	obj.Version = lower(resp.VersionId, "")

	return obj, nil
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

// List returns a list of objects matching the given criteria.
//
// The returned ListResult contains pagination information; use NextToken with
// WithPaginationToken() to fetch the next page. HasMore indicates whether
// additional objects are available.
func (c *Client) List(ctx context.Context, opts ...ClientOption) (*ListResult, error) {
	o := new(ClientOptions).defaults(c.options)

	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.ListObjectsV2(
		ctx,
		&s3.ListObjectsV2Input{
			Bucket:            aws.String(o.BucketName),
			Delimiter:         o.Delimiter,
			Prefix:            o.Prefix,
			MaxKeys:           o.MaxKeys,
			ContinuationToken: o.PaginationToken,
			StartAfter:        o.StartAfter,
		},
		o.S3Options...,
	)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't list %s: %v", o.BucketName, err)
	}

	result := &ListResult{
		Items:   make([]Object, 0, len(resp.Contents)),
		HasMore: lower(resp.IsTruncated, false),
	}

	result.NextToken = lower(resp.NextContinuationToken, "")

	for _, obj := range resp.Contents {
		result.Items = append(result.Items, Object{
			Bucket:       o.BucketName,
			Key:          lower(obj.Key, ""),
			Etag:         lower(obj.ETag, ""),
			Size:         lower(obj.Size, 0),
			LastModified: lower(obj.LastModified, time.Time{}),
		})
	}

	return result, nil
}

// PresignURL generates a presigned URL for the specified HTTP method, key, and expiry duration.
//
// The following HTTP methods are supported:
//   - http.MethodGet: Generate a URL for downloading an object
//   - http.MethodPut: Generate a URL for uploading an object
//   - http.MethodDelete: Generate a URL for deleting an object
//
// For PUT operations, use WithContentType() and WithContentDisposition() to set headers.
//
// The expiry duration must be positive; the returned URL will only be valid for this duration.
func (c *Client) PresignURL(ctx context.Context, method string, key string, expiry time.Duration, opts ...ClientOption) (string, error) {
	// Validate HTTP method
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodDelete:
	default:
		return "", fmt.Errorf("simplestorage: unsupported HTTP method %q for presigned URL (supported: GET, PUT, DELETE)", method)
	}

	// Validate key
	if key == "" {
		return "", fmt.Errorf("simplestorage: key cannot be empty for presigned URL")
	}

	// Validate expiry
	if expiry <= 0 {
		return "", fmt.Errorf("simplestorage: invalid expiry duration %v for presigned URL (must be positive)", expiry)
	}

	// Build options
	o := new(ClientOptions).defaults(c.options)
	for _, doer := range opts {
		doer(&o)
	}

	// Create presign client
	presignClient := s3.NewPresignClient(c.cli.Client)

	// Route to appropriate presign method
	switch method {
	case http.MethodGet:
		return presignURLGet(ctx, presignClient, o.BucketName, key, expiry)
	case http.MethodPut:
		return presignURLPut(ctx, presignClient, o.BucketName, key, expiry, o)
	case http.MethodDelete:
		return presignURLDelete(ctx, presignClient, o.BucketName, key, expiry)
	}

	return "", nil // unreachable
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

// presignURLGet generates a presigned URL for GET operations.
func presignURLGet(ctx context.Context, client *s3.PresignClient, bucket, key string, expiry time.Duration) (string, error) {
	presignResult, err := client.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}

	return presignResult.URL, nil
}

// presignURLPut generates a presigned URL for PUT operations.
func presignURLPut(ctx context.Context, client *s3.PresignClient, bucket, key string, expiry time.Duration, opts ClientOptions) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Apply optional headers
	if opts.ContentType != nil {
		input.ContentType = opts.ContentType
	}
	if opts.ContentDisposition != nil {
		input.ContentDisposition = opts.ContentDisposition
	}

	presignResult, err := client.PresignPutObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}

	return presignResult.URL, nil
}

// presignURLDelete generates a presigned URL for DELETE operations.
func presignURLDelete(ctx context.Context, client *s3.PresignClient, bucket, key string, expiry time.Duration) (string, error) {
	presignResult, err := client.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign delete: %w", err)
	}

	return presignResult.URL, nil
}
