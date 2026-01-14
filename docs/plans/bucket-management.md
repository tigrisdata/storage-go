# Bucket Management for simplestorage

## Overview

Bucket management capabilities were added to the `simplestorage` package while maintaining its core philosophy of **practical simplicity** - minimal cognitive load for users and minimal runtime overhead.

## Design Philosophy

The `simplestorage` package emphasizes:

- **Focused API surface**: Only 4 core object operations (Get, Put, Delete, List)
- **Fixed bucket scoping**: Bucket required at client creation (via env var or option)
- **Consistent patterns**: Error wrapping, functional options, table-driven tests
- **Embedded storage.Client**: Access to Tigris-specific methods via `c.cli`

## Implementation Summary

### New Files

| File                                    | Purpose                                           |
| --------------------------------------- | ------------------------------------------------- |
| `simplestorage/buckets.go`              | Core bucket management methods and response types |
| `simplestorage/bucket_options.go`       | BucketOptions struct and BucketOption functions   |
| `simplestorage/buckets_test.go`         | Table-driven tests for all bucket operations      |
| `simplestorage/buckets_example_test.go` | Godoc examples                                    |

### Modified Files

| File                             | Change                                                           |
| -------------------------------- | ---------------------------------------------------------------- |
| `tigrisheaders/tigrisheaders.go` | Added `WithForkSourceBucket()` and `WithListSnapshots()` helpers |

## API Reference

### Methods (7 new methods on existing `Client`)

```go
// Standard S3 bucket operations
func (c *Client) CreateBucket(ctx context.Context, bucket string, opts ...BucketOption) (*BucketInfo, error)
func (c *Client) DeleteBucket(ctx context.Context, bucket string, opts ...BucketOption) error
func (c *Client) ListBuckets(ctx context.Context, opts ...BucketOption) (*BucketList, error)
func (c *Client) GetBucketInfo(ctx context.Context, bucket string, opts ...BucketOption) (*BucketInfo, error)

// Tigris-specific operations
func (c *Client) CreateBucketSnapshot(ctx context.Context, bucket, description string, opts ...BucketOption) (*SnapshotInfo, error)
func (c *Client) ListBucketSnapshots(ctx context.Context, bucket string, opts ...BucketOption) (*SnapshotList, error)
func (c *Client) ForkBucket(ctx context.Context, source, target string, opts ...BucketOption) (*BucketInfo, error)
```

### Response Types

```go
// BucketInfo contains metadata about a bucket
type BucketInfo struct {
    Name             string    // Bucket name
    Created          time.Time // Creation time

    // Tigris-specific fields
    SnapshotsEnabled bool   // True if snapshots are enabled
    IsForkParent     bool   // True if this bucket has forks
    SourceBucket     string // If this is a fork, the source bucket
    SourceSnapshot   string // If this is a fork, the snapshot version
}

// BucketList contains a paginated list of buckets
type BucketList struct {
    Buckets   []BucketInfo // List of buckets
    NextToken string       // Pagination token for next page
    Truncated bool         // True if more results available
}

// SnapshotInfo contains metadata about a bucket snapshot
type SnapshotInfo struct {
    Name    string    // Snapshot name/description
    Version string    // Snapshot version ID
    Created time.Time // Creation time
    Bucket  string    // Source bucket name
}

// SnapshotList contains a list of snapshots for a bucket
type SnapshotList struct {
    Snapshots []SnapshotInfo // List of snapshots
    Bucket    string         // Source bucket name
}
```

### Options

```go
// BucketOption is a functional option for bucket operations
type BucketOption func(*BucketOptions)

// BucketOptions for bucket-level operations
type BucketOptions struct {
    EnableSnapshot     bool                       // Enable snapshot capability on create
    SnapshotVersion    string                     // Specific snapshot version to target
    ForceDelete        bool                       // Force delete non-empty bucket
    Region             string                     // Static replication region
    S3Options          []func(*s3.Options)        // S3 options passed through
    MaxKeys            *int32                     // Pagination limit
    ContinuationToken  *string                    // Pagination token
}

// Available options
func WithEnableSnapshot() BucketOption
func WithSnapshotVersion(version string) BucketOption
func WithForceDelete() BucketOption
func WithBucketRegion(region string) BucketOption
func WithListLimit(limit int32) BucketOption
func WithListToken(token string) BucketOption
```

### Errors

```go
var (
    ErrBucketNotFound   = errors.New("simplestorage: bucket not found")
    ErrBucketNotEmpty   = errors.New("simplestorage: bucket not empty")
    ErrSnapshotRequired = errors.New("simplestorage: snapshot version required for this operation")
)
```

## Key Design Decisions

### Explicit Bucket Names

Bucket methods require explicit bucket names - **no fallback to client's default bucket**. Bucket management operations assume the caller knows which buckets to operate on:

```go
// Object operations use client's default bucket
client.Get(ctx, "file.txt")  // Uses bucket from client creation

// Bucket management requires explicit bucket name
client.CreateBucket(ctx, "my-new-bucket")  // Must specify bucket
client.DeleteBucket(ctx, "my-new-bucket")  // Must specify bucket
```

This is intentional: bucket management is a separate concern from object operations.

### Storage Client Integration

The implementation uses existing `storage.Client` methods via `c.cli`:

- `c.cli.CreateSnapshotEnabledBucket()` for snapshot-enabled buckets
- `c.cli.CreateBucketSnapshot()` for creating snapshots
- `c.cli.HeadBucketForkOrSnapshot()` for bucket metadata
- `c.cli.ListBuckets()` with Tigris headers for listing snapshots

### Error Handling Pattern

All errors follow the existing pattern:

```go
return nil, fmt.Errorf("simplestorage: can't <verb> <noun>: %w", err)
```

### Pagination for ListBuckets

Uses continuation tokens from AWS SDK:

```go
list, err := client.ListBuckets(ctx,
    simplestorage.WithListLimit(50),
    simplestorage.WithListToken(list.NextToken),
)

if !list.Truncated {
    break  // All results retrieved
}
```

### Force Delete Implementation

Non-empty buckets can be force deleted by emptying them first:

```go
err := client.DeleteBucket(ctx, "my-bucket",
    simplestorage.WithForceDelete(),  // Empties bucket before deleting
)
```

## Tigris-Specific Features

### Bucket Snapshots

Tigris supports point-in-time snapshots of buckets:

```go
// Create a snapshot-enabled bucket
info, err := client.CreateBucket(ctx, "my-bucket",
    simplestorage.WithEnableSnapshot(),
)

// Create a named snapshot
snapshot, err := client.CreateBucketSnapshot(ctx, "my-bucket", "Backup before migration")

// List all snapshots
snapshots, err := client.ListBucketSnapshots(ctx, "my-bucket")
```

### Bucket Forking

Fork a bucket to create an independent copy:

```go
// Fork from current state
forkInfo, err := client.ForkBucket(ctx, "original-bucket", "forked-bucket")

// Fork from a specific snapshot
forkInfo, err = client.ForkBucket(ctx, "original-bucket", "forked-bucket-v2",
    simplestorage.WithSnapshotVersion("snapshot-version-id"),
)
```

### Static Replication Regions

Create buckets with static replication to specific regions:

```go
info, err := client.CreateBucket(ctx, "my-multi-region-bucket",
    simplestorage.WithBucketRegion("fra"),  // Frankfurt, Germany
)
```

Available regions: `fra`, `gru`, `hkg`, `iad`, `jnb`, `lhr`, `mad`, `nrt`, `ord`, `sin`, `sjc`, `syd`, `eur` (aggregate), `usa` (aggregate).

See the [Tigris documentation](https://www.tigrisdata.com/docs/concepts/regions/) for more information.

## Testing

### Test Structure

Tests follow the existing table-driven pattern:

```go
func TestCreateBucket(t *testing.T) {
    tests := []struct {
        name     string
        bucket   string
        options  []BucketOption
        wantErr  error
    }{
        {
            name:    "create snapshot-enabled bucket",
            bucket:  "test-bucket",
            options: []BucketOption{WithEnableSnapshot()},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            skipIfNoCreds(t)  // Guard for integration tests
            // test implementation
        })
    }
}
```

### Environment Variable Guards

Integration tests check for Tigris credentials and skip if not set:

```go
func skipIfNoCreds(t *testing.T) {
    t.Helper()
    if os.Getenv("TIGRIS_STORAGE_ACCESS_KEY_ID") == "" ||
       os.Getenv("TIGRIS_STORAGE_SECRET_ACCESS_KEY") == "" {
        t.Skip("skipping: TIGRIS_STORAGE_ACCESS_KEY_ID and TIGRIS_STORAGE_SECRET_ACCESS_KEY not set")
    }
}
```

### Helper Functions

- `skipIfNoCreds(t)` - Checks for Tigris credentials, skips if missing
- `setupTestBucket(t, ctx, client) string` - Creates bucket, returns name
- `cleanupTestBucket(t, ctx, client, bucket)` - Deletes bucket with force (idempotent)

## New tigrisheaders Helpers

Two new helper functions were added for better ergonomics:

```go
// WithForkSourceBucket sets the source bucket when creating a fork.
func WithForkSourceBucket(sourceBucket string) func(*s3.Options)

// WithListSnapshots lists snapshots for the given bucket.
// This is different from WithTakeSnapshot which creates a snapshot.
func WithListSnapshots(bucketName string) func(*s3.Options)
```

## Examples

### Complete Workflow

```go
ctx := context.Background()

client, err := simplestorage.New(ctx,
    simplestorage.WithBucket("my-default-bucket"),
)

// Create a snapshot-enabled bucket
info, err := client.CreateBucket(ctx, "my-new-bucket",
    simplestorage.WithEnableSnapshot(),
)

// Create a snapshot
snapshot, err := client.CreateBucketSnapshot(ctx, "my-new-bucket", "Initial state")

// Fork from the snapshot
forkInfo, err := client.ForkBucket(ctx, "my-new-bucket", "my-forked-bucket",
    simplestorage.WithSnapshotVersion(snapshot.Version),
)

// Get bucket info
info, err = client.GetBucketInfo(ctx, "my-forked-bucket")

// Clean up
err = client.DeleteBucket(ctx, "my-forked-bucket", simplestorage.WithForceDelete())
err = client.DeleteBucket(ctx, "my-new-bucket", simplestorage.WithForceDelete())
```

## Backward Compatibility

No changes to existing simplestorage files - fully backward compatible. All existing code continues to work without modification.

## Status

- Implementation: Complete
- Tests: Passing
- Documentation: Godoc examples in `buckets_example_test.go`
