package storage_test

import (
	"archive/tar"
	"context"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/joho/godotenv/autoload"
	storage "github.com/tigrisdata/storage-go"
)

func ExampleNew() {
	ctx := context.Background()

	// Create a new Tigris client with default options
	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	_ = client

	// Create a client with custom options
	client, err = storage.New(ctx,
		storage.WithFlyEndpoint(),    // Use fly.io optimized endpoint
		storage.WithGlobalEndpoint(), // Use globally available endpoint (default)
		storage.WithRegion("auto"),   // Specify a region
		// storage.WithAccessKeypair(key, secret), // Set access credentials
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleClient_CreateSnapshotEnabledBucket() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create a bucket with snapshot support enabled
	output, err := client.CreateSnapshotEnabledBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = output
}

func ExampleClient_CreateBucketSnapshot() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create a snapshot with a description
	output, err := client.CreateBucketSnapshot(ctx, "Initial backup", &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = output
}

func ExampleClient_CreateBucketFork() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Creates a new bucket "my-bucket-fork" as a fork of "my-bucket"
	output, err := client.CreateBucketFork(ctx, "my-bucket", "my-bucket-fork")
	if err != nil {
		log.Fatal(err)
	}
	_ = output
}

func ExampleClient_ListBucketSnapshots() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// List all snapshots for a bucket
	snapshots, err := client.ListBucketSnapshots(ctx, "my-bucket")
	if err != nil {
		log.Fatal(err)
	}
	_ = snapshots
}

func ExampleClient_HeadBucketForkOrSnapshot() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Get fork/snapshot metadata for a bucket
	info, err := client.HeadBucketForkOrSnapshot(ctx, &s3.HeadBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}

	_ = info.SnapshotsEnabled     // true if snapshots are enabled
	_ = info.SourceBucket         // The bucket this was forked from
	_ = info.SourceBucketSnapshot // The snapshot this was forked from
	_ = info.IsForkParent         // true if there are forks of this bucket
}

func ExampleClient_RenameObject() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Rename an object in-place without copying data
	_, err = client.RenameObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String("my-bucket"),
		CopySource: aws.String("my-bucket/old-name.txt"),
		Key:        aws.String("new-name.txt"),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_BundleObjects() {
	ctx := context.Background()

	client, err := storage.New(ctx)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Fetch multiple objects as a streaming tar archive in a single request.
	output, err := client.BundleObjects(ctx, &storage.BundleObjectsInput{
		Bucket: "my-dataset-bucket",
		Keys: []string{
			"train/img_001.jpg",
			"train/img_002.jpg",
			"train/img_003.jpg",
		},
	})
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	defer output.Body.Close()

	// Iterate tar entries as they arrive.
	tr := tar.NewReader(output.Body)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err) // handle the error here
		}

		// Read the object content.
		data, err := io.ReadAll(tr)
		if err != nil {
			log.Fatal(err) // handle the error here
		}

		_ = hdr.Name // object key, e.g. "train/img_001.jpg"
		_ = data     // object content
	}
}
