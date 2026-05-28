package simplestorage_test

import (
	"context"
	"fmt"
	"log"

	_ "github.com/joho/godotenv/autoload"
	simplestorage "github.com/tigrisdata/storage-go/simplestorage"
)

func ExampleClient_CreateBucket() {
	ctx := context.Background()

	// Create a simplestorage client (requires TIGRIS_STORAGE_BUCKET env var or WithBucket option)
	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a standard bucket
	info, err := client.CreateBucket(ctx, "my-new-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Created bucket: %s\n", info.Name)
}

func ExampleClient_CreateBucket_snapshot() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a snapshot-enabled bucket (Tigris feature)
	info, err := client.CreateBucket(ctx, "my-snapshot-bucket",
		simplestorage.WithEnableSnapshot(),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Created bucket with snapshots: %s\n", info.Name)
}

func ExampleClient_DeleteBucket() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Delete a bucket (fails if not empty)
	err = client.DeleteBucket(ctx, "my-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleClient_Buckets() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// ListBuckets returns an iterator that transparently handles pagination.
	for bucket, err := range client.Buckets(ctx) {
		if err != nil {
			log.Fatal(err) // handle the error here
		}

		fmt.Printf("Bucket: %s (created: %s)\n", bucket.Name, bucket.Created)
	}
}

func ExampleClient_GetBucketInfo() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Get bucket information
	info, err := client.GetBucketInfo(ctx, "my-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Snapshots enabled: %v\n", info.SnapshotsEnabled)
	fmt.Printf("Is fork parent: %v\n", info.IsForkParent)
	fmt.Printf("Source bucket: %s\n", info.SourceBucket)
	fmt.Printf("Source snapshot: %s\n", info.SourceSnapshot)
}

func ExampleClient_Snapshot() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a named snapshot
	snapshot, err := client.Snapshot(ctx, "my-bucket", "Backup before migration")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Created snapshot: %s (version: %s)\n", snapshot.Name, snapshot.Version)
}

func ExampleClient_Snapshots() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err) // handle error here
	}

	for snapshot, err := range client.Snapshots(ctx, "my-bucket") {
		if err != nil {
			log.Fatal(err) // handle error here
		}
		fmt.Printf("Snapshot: %s (version: %s, created: %s)\n", snapshot.Name, snapshot.Version, snapshot.Created)
	}
}

func ExampleClient_ForkBucket() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Fork a bucket
	forkInfo, err := client.ForkBucket(ctx, "original-bucket", "forked-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Forked bucket: %s (from: %s)\n", forkInfo.Name, forkInfo.SourceBucket)

	// Fork from a specific snapshot version
	forkInfo, err = client.ForkBucket(ctx, "original-bucket", "forked-bucket-v2",
		simplestorage.WithSnapshotVersion("snapshot-version-id"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Forked from snapshot: %s\n", forkInfo.SourceSnapshot)
}

func Example_bucketManagementWorkflow() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new bucket
	info, err := client.CreateBucket(ctx, "my-new-bucket",
		simplestorage.WithEnableSnapshot(),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Create a snapshot
	snapshot, err := client.Snapshot(ctx, "my-new-bucket", "Initial state")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Fork from the snapshot
	forkInfo, err := client.ForkBucket(ctx, "my-new-bucket", "my-forked-bucket",
		simplestorage.WithSnapshotVersion(snapshot.Version),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	_ = forkInfo // Use the fork info

	// Get bucket info
	info, err = client.GetBucketInfo(ctx, "my-forked-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Forked bucket info: %+v\n", info)

	// Clean up - delete both buckets
	// Note: Buckets must be empty before they can be deleted
	err = client.DeleteBucket(ctx, "my-forked-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	err = client.DeleteBucket(ctx, "my-new-bucket")
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleWithBucketRegion() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a bucket with static replication to specific regions
	info, err := client.CreateBucket(ctx, "my-multi-region-bucket",
		simplestorage.WithBucketRegion("fra"), // Frankfurt, Germany
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("Created bucket: %s\n", info.Name)
}
