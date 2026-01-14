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
		log.Fatal(err)
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
		log.Fatal(err)
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
		log.Fatal(err)
	}
}

func ExampleClient_ListBuckets() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// List all buckets
	list, err := client.ListBuckets(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, bucket := range list.Buckets {
		fmt.Printf("Bucket: %s (created: %s)\n", bucket.Name, bucket.Created)
	}

	// Paginated listing
	for {
		list, err = client.ListBuckets(ctx,
			simplestorage.WithListLimit(50),
			simplestorage.WithListToken(list.NextToken),
		)
		if err != nil {
			log.Fatal(err)
		}

		// Process buckets...

		if !list.Truncated {
			break
		}
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
		log.Fatal(err)
	}

	fmt.Printf("Snapshots enabled: %v\n", info.SnapshotsEnabled)
	fmt.Printf("Is fork parent: %v\n", info.IsForkParent)
	fmt.Printf("Source bucket: %s\n", info.SourceBucket)
	fmt.Printf("Source snapshot: %s\n", info.SourceSnapshot)
}

func ExampleClient_CreateBucketSnapshot() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a named snapshot
	snapshot, err := client.CreateBucketSnapshot(ctx, "my-bucket", "Backup before migration")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created snapshot: %s (version: %s)\n", snapshot.Name, snapshot.Version)
}

func ExampleClient_ListBucketSnapshots() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// List all snapshots for a bucket
	snapshots, err := client.ListBucketSnapshots(ctx, "my-bucket")
	if err != nil {
		log.Fatal(err)
	}

	for _, snap := range snapshots.Snapshots {
		fmt.Printf("Snapshot: %s (version: %s, created: %s)\n", snap.Name, snap.Version, snap.Created)
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
		log.Fatal(err)
	}

	fmt.Printf("Forked bucket: %s (from: %s)\n", forkInfo.Name, forkInfo.SourceBucket)

	// Fork from a specific snapshot version
	forkInfo, err = client.ForkBucket(ctx, "original-bucket", "forked-bucket-v2",
		simplestorage.WithSnapshotVersion("snapshot-version-id"),
	)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	// Create a snapshot
	snapshot, err := client.CreateBucketSnapshot(ctx, "my-new-bucket", "Initial state")
	if err != nil {
		log.Fatal(err)
	}

	// Fork from the snapshot
	forkInfo, err := client.ForkBucket(ctx, "my-new-bucket", "my-forked-bucket",
		simplestorage.WithSnapshotVersion(snapshot.Version),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = forkInfo // Use the fork info

	// Get bucket info
	info, err = client.GetBucketInfo(ctx, "my-forked-bucket")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Forked bucket info: %+v\n", info)

	// Clean up - delete both buckets
	// Note: Buckets must be empty before they can be deleted
	err = client.DeleteBucket(ctx, "my-forked-bucket")
	if err != nil {
		log.Fatal(err)
	}

	err = client.DeleteBucket(ctx, "my-new-bucket")
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	fmt.Printf("Created bucket: %s\n", info.Name)
}
