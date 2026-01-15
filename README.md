# Tigris Storage SDK for Go

Welcome to the Tigris Storage SDK for Go! This package contains high-level wrappers and helpers to help you take advantage of all of Tigris' features.

## Overview

[Tigris](https://www.tigrisdata.com/) is a cloud storage service that provides a simple, scalable, and secure object storage solution. It is based on the S3 API, but has additional features that need these helpers.

This SDK provides the main **`storage`** package containing the Tigris client with S3-compatible methods plus Tigris-specific features like bucket forking, snapshots, and object renaming.

## Installation

```bash
go get github.com/tigrisdata/storage-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    storage "github.com/tigrisdata/storage-go"
)

func main() {
    ctx := context.Background()

    // Create a new Tigris client
    client, err := storage.New(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Use the client like you would use AWS S3 client
    // plus Tigris-specific features (see below)
    fmt.Println("Connected to Tigris!")
}
```

## Client Configuration

The `New()` function creates a new S3 client optimized for interactions with Tigris. It accepts functional options to configure the client:

```go
client, err := storage.New(ctx,
    storage.WithFlyEndpoint(),           // Use fly.io optimized endpoint
    storage.WithGlobalEndpoint(),        // Use globally available endpoint (default)
    storage.WithRegion("auto"),          // Specify a region
    storage.WithAccessKeypair(key, secret), // Set access credentials
)
```

### Configuration Options

| Option                                            | Description                                                                                                          |
| ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `WithFlyEndpoint()`                               | Connect to Tigris' fly.io optimized endpoint. If you are deployed to fly.io, this zero-rates your traffic to Tigris. |
| `WithGlobalEndpoint()`                            | Connect to Tigris' globally available endpoint (`https://t3.storage.dev`). This is the default.                      |
| `WithRegion(region)`                              | Statically specify a region for interacting with Tigris. You will almost certainly never need this.                  |
| `WithAccessKeypair(accessKeyID, secretAccessKey)` | Specify a custom access key and secret access key. Useful when loading credentials from non-standard locations.      |

## Bucket Features

### Snapshots and Forks

Tigris supports bucket snapshots and forking, allowing you to create point-in-time copies of buckets and branch from them.

#### Create a Snapshot Enabled Bucket

```go
output, err := client.CreateSnapshotEnabledBucket(ctx, &s3.CreateBucketInput{
    Bucket: aws.String("my-bucket"),
})
```

#### Create a Snapshot

```go
output, err := client.CreateBucketSnapshot(ctx, "Initial backup", &s3.CreateBucketInput{
    Bucket: aws.String("my-bucket"),
})
```

#### Fork a Bucket

```go
// Creates a new bucket "my-bucket-fork" as a fork of "my-bucket"
output, err := client.CreateBucketFork(ctx, "my-bucket", "my-bucket-fork")
```

#### List Snapshots

```go
snapshots, err := client.ListBucketSnapshots(ctx, "my-bucket")
```

#### Get Fork/Snapshot Metadata

```go
info, err := client.HeadBucketForkOrSnapshot(ctx, &s3.HeadBucketInput{
    Bucket: aws.String("my-bucket"),
})
// info.SnapshotsEnabled     - true if snapshots are enabled
// info.SourceBucket         - The bucket this was forked from
// info.SourceBucketSnapshot - The snapshot this was forked from
// info.IsForkParent         - true if there are forks of this bucket
```

## Object Features

### Rename Objects

Tigris supports in-place object renaming without copying data:

```go
_, err := client.RenameObject(ctx, &s3.CopyObjectInput{
    Bucket:     aws.String("my-bucket"),
    CopySource: aws.String("my-bucket/old-name.txt"),
    Key:        aws.String("new-name.txt"),
})
```

## Documentation

For more information on Tigris features, see:

- [Snapshots and Forks](https://www.tigrisdata.com/docs/buckets/snapshots-and-forks/)
- [Object Rename](https://www.tigrisdata.com/docs/objects/object-rename/)
- [Query Metadata](https://www.tigrisdata.com/docs/objects/query-metadata/)
- [Conditional Operations](https://www.tigrisdata.com/docs/objects/conditionals/)
- [Regions](https://www.tigrisdata.com/docs/concepts/regions/)

## License

See [LICENSE](../LICENSE) for details.
