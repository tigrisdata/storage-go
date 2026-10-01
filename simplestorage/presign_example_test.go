package simplestorage_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	simplestorage "github.com/tigrisdata/storage-go/simplestorage"
	"github.com/tigrisdata/storage-go/tigrisheaders"
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
	url, err := client.PresignURL(ctx, http.MethodGet, "documents/report.pdf", time.Hour)
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
	url, err := client.PresignURL(ctx, http.MethodPut, "uploads/avatar.png", 15*time.Minute,
		simplestorage.WithContentType("image/png"),
		simplestorage.WithContentDisposition("attachment"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	// Client can now PUT directly to url
	fmt.Println("Presigned PUT URL:", url)
}

func ExampleClient_PresignURL_signedTigrisHeaders() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Headers passed through WithS3Options are part of the signature. The
	// client using the URL must send them with the same values, or Tigris
	// rejects the upload because the signature will not match.
	url, err := client.PresignURL(ctx, http.MethodPut, "uploads/model.bin", 15*time.Minute,
		simplestorage.WithContentType("application/octet-stream"),
		simplestorage.WithS3Options(
			tigrisheaders.WithStaticReplicationRegions([]tigrisheaders.Region{tigrisheaders.IAD, tigrisheaders.FRA}),
			tigrisheaders.WithHeader("x-amz-meta-owner", "alice"),
		),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader("model bytes"))
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Tigris-Regions", "iad,fra")
	req.Header.Set("x-amz-meta-owner", "alice")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	defer resp.Body.Close()

	fmt.Println("Upload status:", resp.Status)
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
	url, err := client.PresignURL(ctx, http.MethodDelete, "temp/file.txt", 30*time.Minute)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println("Presigned DELETE URL:", url)
}
