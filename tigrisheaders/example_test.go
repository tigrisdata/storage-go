package tigrisheaders_test

import (
	"bytes"
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/joho/godotenv/autoload"
	storage "github.com/tigrisdata/storage-go"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

var client *storage.Client
var ctx context.Context
var data []byte

func ExampleWithStaticReplicationRegions() {
	// Replicate to specific regions
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("my-bucket"),
		Key:    aws.String("file.txt"),
		Body:   bytes.NewReader(data),
	}, tigrisheaders.WithStaticReplicationRegions([]tigrisheaders.Region{
		tigrisheaders.FRA, // Frankfurt
		tigrisheaders.SJC, // San Jose
	}))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithQuery() {
	// Filter objects with a SQL-like WHERE clause
	_, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String("my-bucket"),
	}, tigrisheaders.WithQuery("WHERE `Last-Modified` > \"2023-01-15T08:30:00Z\" AND `Content-Type` = \"text/javascript\""))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithCreateObjectIfNotExists() {
	// Create object only if it doesn't exist
	_, err := client.PutObject(ctx, &s3.PutObjectInput{},
		tigrisheaders.WithCreateObjectIfNotExists())
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithIfEtagMatches() {
	// Only proceed if ETag matches
	_, err := client.PutObject(ctx, &s3.PutObjectInput{},
		tigrisheaders.WithIfEtagMatches(`"abc123"`))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithModifiedSince() {
	// Only proceed if modified since date
	_, err := client.GetObject(ctx, &s3.GetObjectInput{},
		tigrisheaders.WithModifiedSince(time.Now().Add(-24*time.Hour)))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithUnmodifiedSince() {
	// Only proceed if unmodified since date
	_, err := client.GetObject(ctx, &s3.GetObjectInput{},
		tigrisheaders.WithUnmodifiedSince(time.Now().Add(-24*time.Hour)))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithCompareAndSwap() {
	// Compare-and-swap (skip cache, read from designated region)
	_, err := client.GetObject(ctx, &s3.GetObjectInput{},
		tigrisheaders.WithCompareAndSwap())
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithSnapshotVersion() {
	// List objects from a specific snapshot
	_, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String("my-bucket"),
	}, tigrisheaders.WithSnapshotVersion("snapshot-id"))
	if err != nil {
		log.Fatal(err)
	}

	// Get object from a specific snapshot
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("my-bucket"),
		Key:    aws.String("file.txt"),
	}, tigrisheaders.WithSnapshotVersion("snapshot-id"))
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithSoftDelete() {
	// Create a bucket with the default 7-day soft delete retention window
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	}, tigrisheaders.WithSoftDelete())
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Create a bucket with a custom 30-day retention window
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("my-other-bucket"),
	}, tigrisheaders.WithSoftDelete(30))
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleWithForceDelete() {
	// Delete a bucket even when it is not empty. If the bucket has soft delete
	// enabled, it is moved to a recoverable soft-deleted state instead.
	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String("my-bucket"),
	}, tigrisheaders.WithForceDelete())
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleWithHeader() {
	// Set arbitrary HTTP header on request
	_, err := client.PutObject(ctx, &s3.PutObjectInput{},
		tigrisheaders.WithHeader("X-Custom-Header", "value"))
	if err != nil {
		log.Fatal(err)
	}
}
