package simplestorage

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

var (
	// ErrBucketNameRequired is returned when a bucket name is required but not provided.
	ErrBucketNameRequired = errors.New("simplestorage: bucket name required for bucket management operations")

	// ErrBucketNotFound is returned when a bucket operation fails because the bucket doesn't exist.
	ErrBucketNotFound = errors.New("simplestorage: bucket not found")

	// ErrBucketNotEmpty is returned when trying to delete a non-empty bucket.
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
		return nil, ErrBucketNameRequired
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	// Use CreateBucket if no snapshot options, otherwise use Tigris-specific method
	var err error

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		ACL:    bucketACL(o.Access),
	}

	if o.EnableSnapshot {
		_, err = c.cli.CreateSnapshotEnabledBucket(ctx, input, o.S3Options...)
	} else {
		_, err = c.cli.CreateBucket(ctx, input, o.S3Options...)
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
// If the bucket is not empty, returns ErrBucketNotEmpty.
// The bucket must be manually emptied before deletion.
func (c *Client) DeleteBucket(ctx context.Context, bucket string, opts ...BucketOption) error {
	if bucket == "" {
		return ErrBucketNameRequired
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	_, err := c.cli.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	}, o.S3Options...)

	if err != nil {
		// Check if the error is because the bucket is not empty
		// The S3 API error message typically contains "BucketNotEmpty"
		if containsBucketNotEmptyError(err) {
			return fmt.Errorf("simplestorage: can't delete bucket %s: %w", bucket, ErrBucketNotEmpty)
		}
		return fmt.Errorf("simplestorage: can't delete bucket %s: %w", bucket, err)
	}

	return nil
}

// containsBucketNotEmptyError checks if an error indicates a bucket is not empty.
func containsBucketNotEmptyError(err error) bool {
	if err == nil {
		return false
	}
	// Check error message for BucketNotEmpty indicator
	errMsg := err.Error()
	return strings.Contains(errMsg, "BucketNotEmpty") ||
		strings.Contains(errMsg, "bucket not empty") ||
		strings.Contains(errMsg, "NotEmpty")
}

// Buckets lists all buckets that the authenticated user has access to.
//
// This returns an iterator over all of your buckets including Tigris-specific
// metadata about forks and snapshots.
func (c *Client) Buckets(ctx context.Context, opts ...BucketOption) iter.Seq2[*BucketInfo, error] {
	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	const maxBuckets int32 = 50

	return func(yield func(bucketInfo *BucketInfo, err error) bool) {
		continueToken := o.ContinuationToken

		for {
			resp, err := c.cli.ListBuckets(ctx, &s3.ListBucketsInput{
				ContinuationToken: continueToken,
				MaxBuckets:        new(maxBuckets),
			}, o.S3Options...)

			if err != nil {
				yield(nil, err)
				return
			}

			for _, bucket := range resp.Buckets {
				bi, err := c.GetBucketInfo(ctx, *bucket.Name)
				if err != nil {
					if !yield(nil, err) {
						return
					}
					continue
				}

				if !yield(bi, nil) {
					return
				}
			}

			// An empty or absent continuation token means there are no more pages.
			if resp.ContinuationToken == nil || *resp.ContinuationToken == "" {
				return
			}
			continueToken = resp.ContinuationToken
		}
	}
}

// GetBucketInfo retrieves metadata about the bucket with the given name.
//
// This includes Tigris-specific information like whether snapshots are enabled
// and whether the bucket is a fork of another bucket.
func (c *Client) GetBucketInfo(ctx context.Context, bucket string, opts ...BucketOption) (*BucketInfo, error) {
	if bucket == "" {
		return nil, ErrBucketNameRequired
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

	// If Tigris-specific metadata is not available, fall back to basic BucketInfo.
	// This can happen when the bucket doesn't support Tigris features or when
	// called against non-Tigris S3-compatible storage.
	return &BucketInfo{
		Name: bucket,
	}, nil
}

// CreateBucketSnapshot creates a snapshot with the given description for a bucket.
//
// The bucket must have snapshots enabled (created with WithEnableSnapshot()).
func (c *Client) CreateBucketSnapshot(ctx context.Context, bucket, description string, opts ...BucketOption) (*SnapshotInfo, error) {
	if bucket == "" {
		return nil, ErrBucketNameRequired
	}

	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	resp, err := c.cli.CreateBucketSnapshot(ctx, description, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}, o.S3Options...)

	if err != nil {
		return nil, fmt.Errorf("simplestorage: can't create snapshot for bucket %s: %w", bucket, err)
	}

	return &SnapshotInfo{
		Name:    description,
		Version: resp.SnapshotVersion,
		Created: time.Now(),
		Bucket:  bucket,
	}, nil
}

// Snapshots lists all snapshots for the given bucket.
//
// Tigris returns each snapshot as a pseudo-bucket entry whose Name is the
// snapshot version identifier. The user-provided description is not returned
// by the ListBuckets API, so SnapshotInfo.Name is left empty; use the version
// from CreateBucketSnapshot's response if you need to correlate descriptions.
func (c *Client) Snapshots(ctx context.Context, bucket string, opts ...BucketOption) iter.Seq2[*SnapshotInfo, error] {
	o := new(BucketOptions).defaults()
	for _, doer := range opts {
		doer(&o)
	}

	o.S3Options = append(o.S3Options, tigrisheaders.WithHeader("X-Tigris-Snapshot", bucket))

	return func(yield func(snapshotInfo *SnapshotInfo, err error) bool) {
		resp, err := c.cli.ListBuckets(ctx, &s3.ListBucketsInput{}, o.S3Options...)
		if err != nil {
			yield(nil, err)
			return
		}

		for _, snapshot := range resp.Buckets {
			// Tigris encodes each snapshot as a pseudo-bucket whose Name is the
			// snapshot version followed by the description, e.g.
			// "1779907846120844367; name=my+snapshot". Split the two apart and
			// decode the description (spaces are encoded as "+").
			version, desc, _ := strings.Cut(lower(snapshot.Name, ""), "; name=")

			if !yield(&SnapshotInfo{
				Name:    strings.ReplaceAll(desc, "+", " "),
				Version: version,
				Created: lower(snapshot.CreationDate, time.Time{}),
				Bucket:  bucket,
			}, nil) {
				return
			}
		}
	}
}

// ForkBucket creates a fork of the source bucket with the given target name.
//
// Use WithSnapshotVersion() to fork from a specific snapshot version.
func (c *Client) ForkBucket(ctx context.Context, source, target string, opts ...BucketOption) (*BucketInfo, error) {
	if source == "" {
		return nil, fmt.Errorf("simplestorage: source bucket name required: %w", ErrBucketNameRequired)
	}
	if target == "" {
		return nil, fmt.Errorf("simplestorage: target bucket name required: %w", ErrBucketNameRequired)
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
