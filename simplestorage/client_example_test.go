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

	info, err := client.Head(ctx, "reports/q1.pdf")
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Printf("size=%d type=%s\n", info.Size, info.ContentType)
}

func ExampleClient_Head_snapshotVersion() {
	ctx := context.Background()

	client, err := simplestorage.New(ctx,
		simplestorage.WithBucket("my-bucket"),
	)
	if err != nil {
		log.Fatal(err)
	}

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
	obj, err := client.Put(ctx, &simplestorage.Object{
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

	fmt.Println(obj.Etag)
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
	obj, err := client.Put(ctx, &simplestorage.Object{
		Key:  "uploads/image.png",
		Body: io.NopCloser(body),
		Size: int64(body.Len()),
	},
		simplestorage.WithRandomSuffix(),
	)
	if err != nil {
		log.Fatal(err) // handle the error here
	}

	fmt.Println(obj.Key)
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
	for obj, err := range client.List(ctx, simplestorage.WithPrefix("reports/"), simplestorage.WithDelimiter("/")) {
		if err != nil {
			log.Fatal(err) // handle error
		}

		fmt.Println("object:", obj.Key)
	}
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
