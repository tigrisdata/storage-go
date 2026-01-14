package simplestorage

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

// BucketOption is a functional option for bucket management operations.
type BucketOption func(*BucketOptions)

// BucketOptions for bucket-level operations.
type BucketOptions struct {
	// EnableSnapshot enables snapshot capability on bucket creation.
	EnableSnapshot bool

	// SnapshotVersion specifies a snapshot version to target (for forking from specific snapshot).
	SnapshotVersion string

	// Region sets static replication region for the bucket.
	Region string

	// MaxKeys sets the maximum number of results to return in ListBuckets.
	MaxKeys *int32

	// ContinuationToken is the pagination token for ListBuckets.
	ContinuationToken *string

	// S3Options are additional S3 options passed through to the underlying client.
	S3Options []func(*s3.Options)
}

// defaults populates BucketOptions with default values.
func (BucketOptions) defaults() BucketOptions {
	return BucketOptions{
		EnableSnapshot:    false,
		MaxKeys:           nil,
		ContinuationToken: nil,
		S3Options:         []func(*s3.Options){},
	}
}

// WithEnableSnapshot enables snapshot capability when creating a bucket.
func WithEnableSnapshot() BucketOption {
	return func(o *BucketOptions) {
		o.EnableSnapshot = true
		o.S3Options = append(o.S3Options, tigrisheaders.WithEnableSnapshot())
	}
}

// WithSnapshotVersion specifies a snapshot version to target.
// Use this when forking from a specific snapshot version.
func WithSnapshotVersion(version string) BucketOption {
	return func(o *BucketOptions) {
		o.SnapshotVersion = version
		o.S3Options = append(o.S3Options, tigrisheaders.WithSnapshotVersion(version))
	}
}

// WithBucketRegion sets static replication region for the bucket.
//
// For more information, see the Tigris documentation[1].
//
// [1]: https://www.tigrisdata.com/docs/concepts/regions/
func WithBucketRegion(region string) BucketOption {
	return func(o *BucketOptions) {
		o.Region = region
		o.S3Options = append(o.S3Options, tigrisheaders.WithStaticReplicationRegions([]tigrisheaders.Region{tigrisheaders.Region(region)}))
	}
}

// WithListLimit sets the maximum number of buckets to return in ListBuckets.
func WithListLimit(limit int32) BucketOption {
	return func(o *BucketOptions) {
		o.MaxKeys = &limit
	}
}

// WithListToken sets the continuation token for paginated ListBuckets calls.
func WithListToken(token string) BucketOption {
	return func(o *BucketOptions) {
		o.ContinuationToken = &token
	}
}
