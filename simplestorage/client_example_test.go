package simplestorage_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	simplestorage "github.com/tigrisdata/storage-go/simplestorage"
)

func ExampleClient_Head() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Fetch metadata without downloading the body.
	info, err := client.Head(ctx, "reports/q1.pdf")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("size=%d type=%s url=%s\n", info.Size, info.ContentType, info.URL)
}

func ExampleClient_Head_snapshotVersion() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Read metadata from a specific snapshot version.
	info, err := client.Head(ctx, "reports/q1.pdf",
		simplestorage.WithQuerySnapshotVersion("2024-01-01T00:00:00Z"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	_ = info
}

func ExampleClient_Get_responseOverrides() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Force the response Content-Disposition so browsers download rather than render.
	obj, err := client.Get(ctx, "reports/q1.pdf",
		simplestorage.WithResponseContentDisposition(`attachment; filename="q1.pdf"`),
		simplestorage.WithResponseContentType("application/pdf"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	defer obj.Body.Close()

	_, err = io.Copy(io.Discard, obj.Body)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleClient_Put_publicAccess() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	body := strings.NewReader("hello world")
	resp, err := client.Put(ctx, &simplestorage.Object{
		Key:         "public/greeting.txt",
		ContentType: "text/plain",
		Size:        int64(body.Len()),
		Body:        io.NopCloser(body),
	},
		simplestorage.WithAccessType(simplestorage.AccessPublic),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println(resp.URL)
}

func ExampleClient_Put_randomSuffix() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	body := strings.NewReader("payload")
	// The stored key ends up like "uploads/image.png-<random>" so concurrent
	// uploads with the same base name don't collide.
	resp, err := client.Put(ctx, &simplestorage.Object{
		Key:  "uploads/image.png",
		Body: io.NopCloser(body),
		Size: int64(body.Len()),
	},
		simplestorage.WithRandomSuffix(),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println(resp.Path)
}

func ExampleClient_Put_noOverwrite() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	body := strings.NewReader("first write wins")
	_, err = client.Put(ctx, &simplestorage.Object{
		Key:  "config/seed.json",
		Body: io.NopCloser(body),
		Size: int64(body.Len()),
	},
		simplestorage.WithAllowOverwrite(false),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleClient_Put_uploadProgress() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	body := strings.NewReader(strings.Repeat("x", 1024))
	_, err = client.Put(ctx, &simplestorage.Object{
		Key:  "reports/large.bin",
		Body: io.NopCloser(body),
		Size: int64(body.Len()),
	},
		simplestorage.WithUploadProgress(func(p simplestorage.UploadProgress) {
			fmt.Printf("uploaded %d of %d bytes (%.1f%%)\n", p.Loaded, p.Total, p.Percentage)
		}),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
}

func ExampleClient_List_delimiter() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Walk one "directory" level under prefix "reports/" using "/" as a delimiter.
	result, err := client.List(ctx, "reports/",
		simplestorage.WithDelimiter("/"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	for _, p := range result.CommonPrefixes {
		fmt.Println("sub-prefix:", p)
	}
	for _, o := range result.Items {
		fmt.Println("object:", o.Key)
	}
}

func ExampleClient_GetPresignedUrl() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Signed GET URL, valid for 15 minutes.
	res, err := client.GetPresignedUrl(ctx, "reports/q1.pdf",
		simplestorage.WithPresignedExpiresIn(15*60),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println(res.URL)
}

func ExampleClient_GetPresignedUrl_put() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Signed PUT URL that clients can upload to directly with an explicit
	// Content-Type requirement.
	res, err := client.GetPresignedUrl(ctx, "uploads/incoming.bin",
		simplestorage.WithPresignedOperation(simplestorage.PresignOpPut),
		simplestorage.WithPresignedContentType("application/octet-stream"),
		simplestorage.WithPresignedExpiresIn(5*60),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println(res.URL)
}

func ExampleWithBucketAccess() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Public-read bucket: objects inside are world-readable unless overridden.
	info, err := client.CreateBucket(ctx, "public-assets",
		simplestorage.WithBucketAccess(simplestorage.AccessPublic),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	_ = info
}

func ExampleWithDefaultTier() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-default-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Archive-tier bucket for cold storage.
	info, err := client.CreateBucket(ctx, "cold-archive",
		simplestorage.WithDefaultTier("GLACIER"),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}
	_ = info
}
