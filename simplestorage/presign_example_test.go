package simplestorage_test

import (
	"context"
	"fmt"
	"log"
	"time"

	simplestorage "github.com/tigrisdata/storage-go/simplestorage"
)

func ExampleClient_PresignURL_get() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Generate a 1-hour URL for temporary download access
	url, err := client.PresignURL(ctx, "GET", "documents/report.pdf", time.Hour)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println("Presigned GET URL:", url)
}

func ExampleClient_PresignURL_put() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Generate a 15-minute URL for direct upload
	url, err := client.PresignURL(ctx, "PUT", "uploads/avatar.png", 15*time.Minute,
		simplestorage.WithContentType("image/png"),
		simplestorage.WithContentDisposition("attachment"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Client can now PUT directly to url
	fmt.Println("Presigned PUT URL:", url)
}

func ExampleClient_PresignURL_delete() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Generate a 30-minute URL for deletion
	url, err := client.PresignURL(ctx, "DELETE", "temp/file.txt", 30*time.Minute)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println("Presigned DELETE URL:", url)
}
