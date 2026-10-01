package simplestorage

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	storage "github.com/tigrisdata/storage-go"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

func offlinePresignClient(t *testing.T) *Client {
	t.Helper()

	s3cli := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String("https://t3.storage.dev"),
		Credentials:  credentials.NewStaticCredentialsProvider("tid_test", "tsec_test", ""),
	})

	return &Client{cli: &storage.Client{Client: s3cli}, options: Options{BucketName: "presign-test"}}
}

func signedHeaders(t *testing.T, rawURL string) []string {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presigned url: %v", err)
	}

	return strings.Split(u.Query().Get("X-Amz-SignedHeaders"), ";")
}

func TestPresignURLSignsS3OptionHeaders(t *testing.T) {
	cli := offlinePresignClient(t)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			u, err := cli.PresignURL(context.Background(), method, "k", time.Minute,
				WithS3Options(tigrisheaders.WithStaticReplicationRegions([]tigrisheaders.Region{tigrisheaders.IAD, tigrisheaders.FRA})))
			if err != nil {
				t.Fatalf("PresignURL(%s): %v", method, err)
			}

			got := signedHeaders(t, u)
			for _, want := range []string{"host", "x-tigris-regions"} {
				found := false
				for _, h := range got {
					if h == want {
						found = true
					}
				}
				if !found {
					t.Errorf("PresignURL(%s): signed headers %v do not include %q", method, got, want)
				}
			}
		})
	}
}

func TestPresignURLWithoutS3OptionsSignsHostOnly(t *testing.T) {
	cli := offlinePresignClient(t)

	u, err := cli.PresignURL(context.Background(), http.MethodPut, "k", time.Minute)
	if err != nil {
		t.Fatalf("PresignURL: %v", err)
	}

	if got := signedHeaders(t, u); len(got) != 1 || got[0] != "host" {
		t.Errorf("signed headers = %v, want [host]", got)
	}
}
