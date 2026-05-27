package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

// Client is a wrapper around the AWS SDK S3 Client with additional methods for integration with Tigris.
type Client struct {
	*s3.Client
}

// S3 returns the underlying S3 client.
func (c *Client) S3() *s3.Client {
	return c.Client
}

// CreateBucketFork creates a fork of the source bucket named target.
//
// If you want to specify an exact snapshot version to fork from, use tigrisheaders.WithSnapshotVersion.
func (c *Client) CreateBucketFork(ctx context.Context, source, target string, opts ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	opts = append(opts, tigrisheaders.WithHeader("X-Tigris-Fork-Source-Bucket", source))

	return c.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(target),
	}, opts...)
}

// CreateBucketSnapshotOutput is the response from CreateBucketSnapshot.
// It wraps the underlying s3.CreateBucketOutput and surfaces the snapshot
// version that Tigris returns in the X-Tigris-Snapshot-Version response header.
type CreateBucketSnapshotOutput struct {
	*s3.CreateBucketOutput

	// SnapshotVersion is the version identifier of the snapshot just created.
	// Empty if the server did not return a version header.
	SnapshotVersion string
}

// CreateBucketSnapshot creates a snapshot with the given description for a bucket.
// The returned output carries the snapshot version in SnapshotVersion.
func (c *Client) CreateBucketSnapshot(ctx context.Context, description string, in *s3.CreateBucketInput, opts ...func(*s3.Options)) (*CreateBucketSnapshotOutput, error) {
	opts = append(opts, tigrisheaders.WithTakeSnapshot(description))

	resp, err := c.Client.CreateBucket(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	out := &CreateBucketSnapshotOutput{CreateBucketOutput: resp}
	if rawResp, ok := middleware.GetRawResponse(resp.ResultMetadata).(*smithyhttp.Response); ok {
		out.SnapshotVersion = rawResp.Header.Get("X-Tigris-Snapshot-Version")
	}
	return out, nil
}

// CreateSnapshotEnabledBucket creates a new bucket with the ability to take snapshots and fork the contents of it.
func (c *Client) CreateSnapshotEnabledBucket(ctx context.Context, in *s3.CreateBucketInput, opts ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	opts = append(opts, tigrisheaders.WithEnableSnapshot())

	return c.Client.CreateBucket(ctx, in, opts...)
}

// HeadBucketForkOrSnapshotOutput records the fork/snapshot metadata for a bucket.
type HeadBucketForkOrSnapshotOutput struct {
	SnapshotsEnabled     bool   // true if snapshots are enabled, otherwise false.
	SourceBucket         string // The name of the bucket this bucket was forked from.
	SourceBucketSnapshot string // The snapshot this bucket was forked from.
	IsForkParent         bool   // true if there are forks of this bucket, otherwise false.
}

// HeadBucketForkOrSnapshot fetches the fork/snapshot metadata for a bucket.
//
// For more information, see the Tigris documentation[1].
//
// [1]: https://www.tigrisdata.com/docs/buckets/snapshots-and-forks/#retrieving-snapshot-and-fork-info-for-a-bucket
func (c *Client) HeadBucketForkOrSnapshot(ctx context.Context, in *s3.HeadBucketInput, opts ...func(*s3.Options)) (*HeadBucketForkOrSnapshotOutput, error) {
	resp, err := c.Client.HeadBucket(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	rawResp, ok := middleware.GetRawResponse(resp.ResultMetadata).(*smithyhttp.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from middleware")
	}
	return &HeadBucketForkOrSnapshotOutput{
		SnapshotsEnabled:     rawResp.Header.Get("X-Tigris-Enable-Snapshot") == "true",
		SourceBucket:         rawResp.Header.Get("X-Tigris-Fork-Source-Bucket"),
		SourceBucketSnapshot: rawResp.Header.Get("X-Tigris-Fork-Source-Bucket-Snapshot"),
		IsForkParent:         rawResp.Header.Get("X-Tigris-Is-Fork-Parent") == "true",
	}, nil
}

// ListBucketSnapshots lists the snapshots for a bucket.
//
// For more information, see the Tigris documentation[1].
//
// [1]: https://www.tigrisdata.com/docs/buckets/snapshots-and-forks/#listing-snapshots
func (c *Client) ListBucketSnapshots(ctx context.Context, bucketName string, opts ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	opts = append(opts, tigrisheaders.WithHeader("X-Tigris-Snapshot", bucketName))

	return c.Client.ListBuckets(ctx, &s3.ListBucketsInput{}, opts...)
}

// RenameObject performs an in-place rename of objects instead of copying the data.
//
// For more information, see the Tigris documentation[1].
//
// [1]: https://www.tigrisdata.com/docs/objects/object-rename/
func (c *Client) RenameObject(ctx context.Context, in *s3.CopyObjectInput, opts ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	opts = append(opts, tigrisheaders.WithRename())

	return c.Client.CopyObject(ctx, in, opts...)
}
