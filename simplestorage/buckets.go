package simplestorage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

var (
	// ErrBucketNotFound is returned when a bucket operation fails because the bucket doesn't exist.
	ErrBucketNotFound = errors.New("simplestorage: bucket not found")

	// ErrBucketNotEmpty is returned when trying to delete a non-empty bucket without ForceDelete.
	ErrBucketNotEmpty = errors.New("simplestorage: bucket not empty")

	// ErrSnapshotRequired is returned when a snapshot version is required but not provided.
	ErrSnapshotRequired = errors.New("simplestorage: snapshot version required for this operation")
)

// BucketInfo contains metadata about a bucket.
type BucketInfo struct {
	Name    string    // Bucket name
	Created time.Time // Creation time

	// Tigris-specific fields
	SnapshotsEnabled bool   // True if snapshots are enabled
	IsForkParent     bool   // True if this bucket has forks
	SourceBucket     string // If this is a fork, the source bucket
	SourceSnapshot   string // If this is a fork, the snapshot version
}

// BucketList contains a paginated list of buckets.
type BucketList struct {
	Buckets   []BucketInfo // List of buckets
	NextToken string       // Pagination token for next page
	Truncated bool         // True if more results available
}

// SnapshotInfo contains metadata about a bucket snapshot.
type SnapshotInfo struct {
	Name    string    // Snapshot name/description
	Version string    // Snapshot version ID
	Created time.Time // Creation time
	Bucket  string    // Source bucket name
}

// SnapshotList contains a list of snapshots for a bucket.
type SnapshotList struct {
	Snapshots []SnapshotInfo // List of snapshots
	Bucket    string         // Source bucket name
}

// CreateBucket creates a new bucket with the given name.
//
// For Tigris-specific features like snapshots, use options like WithEnableSnapshot().
func (c *Client) CreateBucket(ctx context.Context, bucket string, opts ...BucketOption) (*BucketInfo, error) {
	if bucket == "" {
		return nil, errors.New("simplestorage: bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// Use CreateBucket if no snapshot options, otherwise use Tigris-specific method
	var err error

	if o.EnableSnapshot {
		_, err = c.cli.CreateSnapshotEnabledBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}, o.S3Options...)
	} else {
		_, err = c.cli.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		}, o.S3Options...)
	}

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't create bucket %s: %w", bucket, err)
	}

	return &BucketInfo{
		Name:    bucket,
		Created: time.Now(), // AWS SDK doesn't return creation time in CreateBucket
	}, nil
}

// DeleteBucket deletes the bucket with the given name.
//
// If the bucket is not empty, the operation will fail unless WithForceDelete() is used.
func (c *Client) DeleteBucket(ctx context.Context, bucket string, opts ...BucketOption) error {
	if bucket == "" {
		return errors.New("simplestorage: bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// If force delete, empty the bucket first
	if o.ForceDelete {
		if err := c.emptyBucket(ctx, bucket, o); err != nil {
			return fmt.Errorf("simplestorage: can't empty bucket %s: %w", bucket, err)
		}
	}

	_, err := c.cli.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	}, o.S3Options...)

	if err != nil {
		return fmt.Errorf("simplestorage: can't delete bucket %s: %w", bucket, err)
	}

	return nil
}

// emptyBucket empties a bucket by deleting all objects in it.
func (c *Client) emptyBucket(ctx context.Context, bucket string, o BucketOptions) error {
	// List all objects
	listResp, err := c.cli.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}, o.S3Options...)
	if err != nil {
		return fmt.Errorf("can't list objects: %w", err)
	}

	// Delete each object
	for _, obj := range listResp.Contents {
		_, err := c.cli.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    obj.Key,
		}, o.S3Options...)
		if err != nil {
			return fmt.Errorf("can't delete object %s: %w", *obj.Key, err)
		}
	}

	return nil
}

// ListBuckets lists all buckets that the authenticated user has access to.
//
// Use WithListLimit() and WithListToken() for pagination.
func (c *Client) ListBuckets(ctx context.Context, opts ...BucketOption) (*BucketList, error) {
	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.ListBuckets(ctx, &s3.ListBucketsInput{
		ContinuationToken: o.ContinuationToken,
	}, o.S3Options...)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't list buckets: %w", err)
	}

	result := &BucketList{
		Buckets:   make([]BucketInfo, 0, len(resp.Buckets)),
		Truncated: resp.ContinuationToken != nil,
	}

	for _, b := range resp.Buckets {
		result.Buckets = append(result.Buckets, BucketInfo{
			Name:    *b.Name,
			Created: lower(b.CreationDate, time.Time{}),
		})
	}

	if resp.ContinuationToken != nil {
		result.NextToken = *resp.ContinuationToken
	}

	return result, nil
}

// GetBucketInfo retrieves metadata about the bucket with the given name.
//
// This includes Tigris-specific information like whether snapshots are enabled
// and whether the bucket is a fork of another bucket.
func (c *Client) GetBucketInfo(ctx context.Context, bucket string, opts ...BucketOption) (*BucketInfo, error) {
	if bucket == "" {
		return nil, errors.New("simplestorage: bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// Try Tigris-specific metadata first
	tigrisInfo, err := c.cli.HeadBucketForkOrSnapshot(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}, o.S3Options...)

	if err == nil {
		return &BucketInfo{
			Name:             bucket,
			SnapshotsEnabled: tigrisInfo.SnapshotsEnabled,
			IsForkParent:     tigrisInfo.IsForkParent,
			SourceBucket:     tigrisInfo.SourceBucket,
			SourceSnapshot:   tigrisInfo.SourceBucketSnapshot,
		}, nil
	}

	// If Tigris headers failed, return a basic BucketInfo anyway
	return &BucketInfo{
		Name: bucket,
	}, nil
}

// CreateBucketSnapshot creates a snapshot with the given description for a bucket.
//
// The bucket must have snapshots enabled (created with WithEnableSnapshot()).
func (c *Client) CreateBucketSnapshot(ctx context.Context, bucket, description string, opts ...BucketOption) (*SnapshotInfo, error) {
	if bucket == "" {
		return nil, errors.New("simplestorage: bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// CreateBucketSnapshot uses CreateBucket with snapshot header
	_, err := c.cli.CreateBucketSnapshot(ctx, description, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}, o.S3Options...)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't create snapshot for bucket %s: %w", bucket, err)
	}

	// Note: The snapshot version is returned in HTTP headers that are not directly
	// accessible through the AWS SDK response. Users can list snapshots to get the version.
	return &SnapshotInfo{
		Name:    description,
		Version: "",
		Created: time.Now(),
		Bucket:  bucket,
	}, nil
}

// ListBucketSnapshots lists all snapshots for the given bucket.
func (c *Client) ListBucketSnapshots(ctx context.Context, bucket string, opts ...BucketOption) (*SnapshotList, error) {
	if bucket == "" {
		return nil, errors.New("simplestorage: bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// Use the new tigrisheaders helper
	o.S3Options = append(o.S3Options, tigrisheaders.WithListSnapshots(bucket))

	resp, err := c.cli.ListBuckets(ctx, &s3.ListBucketsInput{}, o.S3Options...)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't list snapshots for bucket %s: %w", bucket, err)
	}

	result := &SnapshotList{
		Bucket:    bucket,
		Snapshots: make([]SnapshotInfo, 0),
	}

	// Parse snapshot info from response buckets
	for _, b := range resp.Buckets {
		// Extract snapshot info from bucket metadata
		// Snapshot names are encoded in bucket names
		snap := SnapshotInfo{
			Name:    lower(b.Name, ""),
			Version: lower(b.Name, ""),
			Created: lower(b.CreationDate, time.Time{}),
			Bucket:  bucket,
		}
		result.Snapshots = append(result.Snapshots, snap)
	}

	return result, nil
}

// ForkBucket creates a fork of the source bucket with the given target name.
//
// Use WithSnapshotVersion() to fork from a specific snapshot version.
func (c *Client) ForkBucket(ctx context.Context, source, target string, opts ...BucketOption) (*BucketInfo, error) {
	if source == "" {
		return nil, errors.New("simplestorage: source bucket name required for bucket management operations")
	}
	if target == "" {
		return nil, errors.New("simplestorage: target bucket name required for bucket management operations")
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// Add fork source bucket to options
	o.S3Options = append(o.S3Options, tigrisheaders.WithForkSourceBucket(source))

	_, err := c.cli.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(target),
	}, o.S3Options...)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't fork bucket %s to %s: %w", source, target, err)
	}

	return &BucketInfo{
		Name:             target,
		Created:          time.Now(),
		SourceBucket:     source,
		SourceSnapshot:   o.SnapshotVersion,
		SnapshotsEnabled: false, // Will be populated if queried via GetBucketInfo
	}, nil
}
