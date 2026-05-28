package simplestorage

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

	// SourceBucketSnapshot specifies the snapshot version to fork from.
	SourceBucketSnapshot string

	// Region sets static replication region for the bucket.
	// This field is stored for visibility but the actual behavior is configured
	// via S3Options (see WithBucketRegion). Keeping the field enables debugging
	// and potential future use in bucket info responses.
	Region string

	// DefaultTier sets the storage class tier for the bucket.
	DefaultTier string

	// Consistency sets the consistency level for the bucket ("strict" or "default").
	Consistency string

	// Access sets the bucket-level access type (public or private).
	Access AccessType

	// MaxKeys sets the maximum number of results to return in ListBuckets.
	MaxKeys *int32

	// ContinuationToken is the pagination token for ListBuckets.
	ContinuationToken *string

	// S3Options are additional S3 options passed through to the underlying client.
	S3Options []func(*s3.Options)

	// GrabForkInfo makes Buckets calls grab additional information about buckets from
	// Tigris about forkability and what snapshot the bucket was forked from.
	GrabForkInfo bool
}

// defaults populates BucketOptions with default values.
func (BucketOptions) defaults() BucketOptions {
	return BucketOptions{
		EnableSnapshot:    false,
		Access:            AccessPrivate,
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

// WithDefaultTier sets the storage class tier for the bucket.
// Valid values: "STANDARD", "STANDARD_IA", "GLACIER", "GLACIER_IR"
func WithDefaultTier(tier string) BucketOption {
	return func(o *BucketOptions) {
		o.DefaultTier = tier
		o.S3Options = append(o.S3Options, tigrisheaders.WithStorageClass(tier))
	}
}

// WithConsistentRead enables consistent read mode for the bucket.
func WithConsistentRead() BucketOption {
	return func(o *BucketOptions) {
		o.Consistency = "strict"
		o.S3Options = append(o.S3Options, tigrisheaders.WithConsistentRead())
	}
}

// WithForkSourceSnapshot specifies the snapshot version when forking from a bucket.
// Use this with CreateBucket when forking from a specific snapshot version.
func WithForkSourceSnapshot(snapshot string) BucketOption {
	return func(o *BucketOptions) {
		o.SourceBucketSnapshot = snapshot
		o.S3Options = append(o.S3Options, tigrisheaders.WithForkSourceBucketSnapshot(snapshot))
	}
}

// WithBucketAccess sets the bucket-level canned ACL (public or private).
// Use AccessPublic to allow anonymous reads via public-read, or AccessPrivate
// (the default) to require authenticated access.
func WithBucketAccess(access AccessType) BucketOption {
	return func(o *BucketOptions) {
		o.Access = access
	}
}

// WithGrabForkInfo instructs the Buckets() call to grab additional information about
// bucket forkability and the snapshot each bucket was based on.
//
// Using this will incur an additional Tigris round trip per invocation.
func WithGrabForkInfo() BucketOption {
	return func(o *BucketOptions) {
		o.GrabForkInfo = true
	}
}

// bucketACL maps an AccessType to an S3 canned ACL for bucket operations.
// Defaults to private for unset values so callers don't accidentally inherit
// a permissive account-wide default.
func bucketACL(a AccessType) s3types.BucketCannedACL {
	switch a {
	case AccessPublic:
		return s3types.BucketCannedACLPublicRead
	default:
		return s3types.BucketCannedACLPrivate
	}
}
