package simplestorage

import "github.com/aws/aws-sdk-go-v2/service/s3"

type listOptions struct {
	// ContinueToken is the continuation token for this request
	ContinueToken *string

	// Delimiter sets the listing delimiter for this request.
	Delimiter *string

	// MaxKeys sets the maximum number of keys to return per loop iteration.
	MaxKeys *int32

	// Prefix sets the key prefix for this request.
	Prefix *string

	// StartAfter is where you want Tigris to start listing from in the bucket.
	// This can be any key in the bucket.
	StartAfter *string

	// S3Options are middleware functions forwarded to the underlying
	// ListObjectsV2 call. Use them for Tigris-specific headers such as
	// tigrisheaders.WithSnapshotVersion.
	S3Options []func(*s3.Options)
}

// ListOption configures a single List call. Pass any combination of these to
// override the default listing behavior.
type ListOption func(*listOptions)

// WithContinueToken resumes a previous List call from the given continuation
// token. Use the token returned by the prior page to fetch the next one.
func WithContinueToken(token string) ListOption {
	return func(li *listOptions) {
		li.ContinueToken = new(token)
	}
}

// WithDelimiter groups keys that share a common prefix up to the given
// delimiter, collapsing them into a single result. The classic value is "/" to
// emulate directory-style listings.
func WithDelimiter(delimiter string) ListOption {
	return func(li *listOptions) {
		li.Delimiter = new(delimiter)
	}
}

// WithMaxKeys caps the number of keys returned per underlying request. This
// controls page size, not the total number of keys yielded by the iterator.
func WithMaxKeys(maxKeys int32) ListOption {
	return func(li *listOptions) {
		li.MaxKeys = new(maxKeys)
	}
}

// WithPrefix restricts the listing to keys that begin with the given prefix.
func WithPrefix(prefix string) ListOption {
	return func(li *listOptions) {
		li.Prefix = new(prefix)
	}
}

// WithStartAfter begins the listing immediately after the given key. The key
// itself does not need to exist in the bucket; Tigris returns the next key in
// lexicographic order.
func WithStartAfter(key string) ListOption {
	return func(li *listOptions) {
		li.StartAfter = new(key)
	}
}

// WithListS3Options appends middleware to the underlying ListObjectsV2 call.
// Use this to apply Tigris-specific headers to a List call, for example reading
// from a bucket snapshot:
//
//	for obj, err := range client.List(ctx,
//		simplestorage.WithListS3Options(tigrisheaders.WithSnapshotVersion("v1")),
//	) {
//		// ...
//	}
func WithListS3Options(opts ...func(*s3.Options)) ListOption {
	return func(li *listOptions) {
		li.S3Options = append(li.S3Options, opts...)
	}
}
