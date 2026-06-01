package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const sampleListVersionsXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <IsTruncated>true</IsTruncated>
  <NextKeyMarker>next-key</NextKeyMarker>
  <NextVersionIdMarker>next-version</NextVersionIdMarker>
  <DeleteMarker>
    <Key>reports/q1.pdf</Key>
    <VersionId>1775929768707198086</VersionId>
    <Size>2048</Size>
    <ETag>"abc123"</ETag>
    <SoftDeleted>true</SoftDeleted>
    <LastModified>2026-05-01T12:30:00.000Z</LastModified>
  </DeleteMarker>
  <DeleteMarker>
    <Key>reports/q2.pdf</Key>
    <VersionId>1775929768707198087</VersionId>
    <Size>4096</Size>
    <ETag>"def456"</ETag>
    <SoftDeleted>true</SoftDeleted>
    <LastModified>2026-05-02T08:15:00.000Z</LastModified>
  </DeleteMarker>
</ListVersionsResult>`

func TestListSoftDeletedObjects_RequestAndParse(t *testing.T) {
	var req *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		_, _ = w.Write([]byte(sampleListVersionsXML))
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
		WithPathStyle(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	out, err := cli.ListSoftDeletedObjects(context.Background(), &ListSoftDeletedObjectsInput{
		Bucket:    "my-bucket",
		Prefix:    "reports/",
		KeyMarker: "start",
		MaxKeys:   100,
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.URL.Path != "/my-bucket" {
		t.Errorf("path = %q, want /my-bucket", req.URL.Path)
	}
	if !req.URL.Query().Has("versions") {
		t.Error("missing ?versions query parameter")
	}
	if got := req.URL.Query().Get("prefix"); got != "reports/" {
		t.Errorf("prefix = %q, want reports/", got)
	}
	if got := req.URL.Query().Get("key-marker"); got != "start" {
		t.Errorf("key-marker = %q, want start", got)
	}
	if got := req.URL.Query().Get("max-keys"); got != "100" {
		t.Errorf("max-keys = %q, want 100", got)
	}
	if got := req.Header.Get("X-Tigris-Soft-Delete"); got != "true" {
		t.Errorf("X-Tigris-Soft-Delete = %q, want true", got)
	}
	if req.Header.Get("Authorization") == "" {
		t.Error("missing Authorization header (SigV4)")
	}

	if len(out.Objects) != 2 {
		t.Fatalf("got %d objects, want 2", len(out.Objects))
	}
	if !out.IsTruncated || out.NextKeyMarker != "next-key" || out.NextVersionIDMarker != "next-version" {
		t.Errorf("pagination fields = %+v", out)
	}
	first := out.Objects[0]
	if first.Key != "reports/q1.pdf" || first.VersionID != "1775929768707198086" {
		t.Errorf("first object key/version = %q/%q", first.Key, first.VersionID)
	}
	if first.Size != 2048 || first.ETag != `"abc123"` || !first.SoftDeleted {
		t.Errorf("first object size/etag/softdeleted = %d/%q/%v", first.Size, first.ETag, first.SoftDeleted)
	}
	want := time.Date(2026, 5, 1, 12, 30, 0, 0, time.UTC)
	if !first.LastModified.Equal(want) {
		t.Errorf("first object LastModified = %v, want %v", first.LastModified, want)
	}
}

func TestListSoftDeletedObjects_Validation(t *testing.T) {
	cli := &Client{}
	if _, err := cli.ListSoftDeletedObjects(context.Background(), &ListSoftDeletedObjectsInput{}); !errors.Is(err, ErrMissingBucket) {
		t.Fatalf("err = %v, want ErrMissingBucket", err)
	}
}

func TestRestoreSoftDeletedObject_RequestConstruction(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		versionID   string
		wantPath    string
		wantVersion string
	}{
		{"latest", "my-key", "", "/my-bucket/my-key", ""},
		{"specific version", "my-key", "1775929768707198086", "/my-bucket/my-key", "1775929768707198086"},
		{"key with slash and space", "folder/my file.txt", "", "/my-bucket/folder/my file.txt", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req = r
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cli, err := New(context.Background(),
				WithEndpoint(server.URL),
				WithAccessKeypair("test-key", "test-secret"),
			)
			if err != nil {
				t.Fatal(err)
			}

			if err := cli.RestoreSoftDeletedObject(context.Background(), "my-bucket", tt.key, tt.versionID); err != nil {
				t.Fatal(err)
			}

			if req.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", req.Method)
			}
			if req.URL.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", req.URL.Path, tt.wantPath)
			}
			if !req.URL.Query().Has("restore") {
				t.Error("missing ?restore query parameter")
			}
			if got := req.Header.Get("X-Tigris-Restore-Type"); got != "soft-delete" {
				t.Errorf("X-Tigris-Restore-Type = %q, want soft-delete", got)
			}
			if got := req.Header.Get("X-Tigris-Restore-Version"); got != tt.wantVersion {
				t.Errorf("X-Tigris-Restore-Version = %q, want %q", got, tt.wantVersion)
			}
			if req.Header.Get("Authorization") == "" {
				t.Error("missing Authorization header (SigV4)")
			}
		})
	}
}

func TestRestoreSoftDeletedObject_Validation(t *testing.T) {
	cli := &Client{}
	if err := cli.RestoreSoftDeletedObject(context.Background(), "", "key", ""); !errors.Is(err, ErrMissingBucket) {
		t.Errorf("err = %v, want ErrMissingBucket", err)
	}
	if err := cli.RestoreSoftDeletedObject(context.Background(), "bucket", "", ""); !errors.Is(err, ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey", err)
	}
}

func TestPermanentlyDeleteObject_Validation(t *testing.T) {
	cli := &Client{}
	if _, err := cli.PermanentlyDeleteObject(context.Background(), "", "key", "v1"); !errors.Is(err, ErrMissingBucket) {
		t.Errorf("err = %v, want ErrMissingBucket", err)
	}
	if _, err := cli.PermanentlyDeleteObject(context.Background(), "bucket", "", "v1"); !errors.Is(err, ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey", err)
	}
	if _, err := cli.PermanentlyDeleteObject(context.Background(), "bucket", "key", ""); !errors.Is(err, ErrMissingVersionID) {
		t.Errorf("err = %v, want ErrMissingVersionID", err)
	}
}

func TestRestoreBucket_RequestConstruction(t *testing.T) {
	var req *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := cli.RestoreBucket(context.Background(), "my-bucket"); err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if req.URL.Path != "/my-bucket" {
		t.Errorf("path = %q, want /my-bucket", req.URL.Path)
	}
	if !req.URL.Query().Has("restore") {
		t.Error("missing ?restore query parameter")
	}
	if req.Header.Get("Authorization") == "" {
		t.Error("missing Authorization header (SigV4)")
	}
}

func TestSetBucketSoftDelete_RequestConstruction(t *testing.T) {
	tests := []struct {
		name              string
		enabled           bool
		retentionDays     int
		wantEnabled       bool
		wantRetentionDays int // 0 means the field should be omitted
	}{
		{"enable default", true, 0, true, 0},
		{"enable custom", true, 30, true, 30},
		{"disable", false, 30, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var body []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req = r
				body, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cli, err := New(context.Background(),
				WithEndpoint(server.URL),
				WithAccessKeypair("test-key", "test-secret"),
			)
			if err != nil {
				t.Fatal(err)
			}

			if err := cli.SetBucketSoftDelete(context.Background(), "my-bucket", tt.enabled, tt.retentionDays); err != nil {
				t.Fatal(err)
			}

			if req.Method != http.MethodPatch {
				t.Errorf("method = %q, want PATCH", req.Method)
			}
			if req.URL.Path != "/my-bucket" {
				t.Errorf("path = %q, want /my-bucket", req.URL.Path)
			}
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("content-type = %q, want application/json", got)
			}

			var parsed patchBucketBody
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("failed to unmarshal body %q: %v", body, err)
			}
			if parsed.SoftDelete.Enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", parsed.SoftDelete.Enabled, tt.wantEnabled)
			}
			if parsed.SoftDelete.RetentionDays != tt.wantRetentionDays {
				t.Errorf("retention_days = %d, want %d", parsed.SoftDelete.RetentionDays, tt.wantRetentionDays)
			}

			// Confirm retention_days is physically absent from the JSON when omitted.
			hasRetention := jsonHasKey(t, body, "retention_days")
			if tt.wantRetentionDays == 0 && hasRetention {
				t.Error("retention_days should be omitted from JSON when 0")
			}
			if tt.wantRetentionDays != 0 && !hasRetention {
				t.Error("retention_days should be present in JSON when set")
			}
		})
	}
}

func TestSetBucketSoftDelete_Validation(t *testing.T) {
	cli := &Client{}
	if err := cli.SetBucketSoftDelete(context.Background(), "", true, 0); !errors.Is(err, ErrMissingBucket) {
		t.Errorf("err = %v, want ErrMissingBucket", err)
	}
}

func TestForceDeleteBucket_RequestConstruction(t *testing.T) {
	var req *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
		WithPathStyle(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cli.ForceDeleteBucket(context.Background(), &s3.DeleteBucketInput{
		Bucket: aws.String("my-bucket"),
	}); err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
	if got := req.Header.Get("X-Tigris-Force-Delete"); got != "true" {
		t.Errorf("X-Tigris-Force-Delete = %q, want true", got)
	}
}

func TestPermanentlyDeleteObject_RequestConstruction(t *testing.T) {
	var req *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
		WithPathStyle(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cli.PermanentlyDeleteObject(context.Background(), "my-bucket", "my-key", "1775929768707198086"); err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
	if req.URL.Path != "/my-bucket/my-key" {
		t.Errorf("path = %q, want /my-bucket/my-key", req.URL.Path)
	}
	if got := req.URL.Query().Get("versionId"); got != "1775929768707198086" {
		t.Errorf("versionId = %q, want 1775929768707198086", got)
	}
	if got := req.Header.Get("X-Tigris-Soft-Delete"); got != "true" {
		t.Errorf("X-Tigris-Soft-Delete = %q, want true", got)
	}
}

func TestCreateBucketWithSoftDelete_RequestConstruction(t *testing.T) {
	tests := []struct {
		name          string
		retentionDays int
		wantHeader    string
	}{
		{"default window", 0, "true"},
		{"custom window", 30, "30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req = r
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cli, err := New(context.Background(),
				WithEndpoint(server.URL),
				WithAccessKeypair("test-key", "test-secret"),
				WithPathStyle(true),
			)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := cli.CreateBucketWithSoftDelete(context.Background(), &s3.CreateBucketInput{
				Bucket: aws.String("my-bucket"),
			}, tt.retentionDays); err != nil {
				t.Fatal(err)
			}

			if req.Method != http.MethodPut {
				t.Errorf("method = %q, want PUT", req.Method)
			}
			if got := req.Header.Get("X-Tigris-Soft-Delete"); got != tt.wantHeader {
				t.Errorf("X-Tigris-Soft-Delete = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestRestoreBucket_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code></Error>`))
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := cli.RestoreBucket(context.Background(), "my-bucket"); err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func jsonHasKey(t *testing.T, body []byte, key string) bool {
	t.Helper()
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	sd, ok := generic["soft_delete"]
	if !ok {
		t.Fatal("body missing soft_delete object")
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(sd, &inner); err != nil {
		t.Fatalf("failed to unmarshal soft_delete: %v", err)
	}
	_, present := inner[key]
	return present
}

// skipIfNoCreds skips integration tests when Tigris credentials are absent.
func skipIfNoCreds(t *testing.T) {
	t.Helper()
	if os.Getenv("TIGRIS_STORAGE_ACCESS_KEY_ID") == "" ||
		os.Getenv("TIGRIS_STORAGE_SECRET_ACCESS_KEY") == "" {
		t.Skip("skipping: TIGRIS_STORAGE_ACCESS_KEY_ID and TIGRIS_STORAGE_SECRET_ACCESS_KEY not set")
	}
}

// TestSoftDeleteLifecycle_integration exercises the full soft-delete lifecycle
// against a live Tigris endpoint when credentials are available.
func TestSoftDeleteLifecycle_integration(t *testing.T) {
	skipIfNoCreds(t)

	ctx := context.Background()
	cli, err := New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	bucket := "storage-go-softdelete-test-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	if _, err := cli.CreateBucketWithSoftDelete(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}, 7); err != nil {
		t.Fatalf("CreateBucketWithSoftDelete: %v", err)
	}
	t.Cleanup(func() {
		_, _ = cli.ForceDeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	key := "hello.txt"
	if _, err := cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("hello world"),
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	if _, err := cli.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	listed, err := cli.ListSoftDeletedObjects(ctx, &ListSoftDeletedObjectsInput{Bucket: bucket})
	if err != nil {
		t.Fatalf("ListSoftDeletedObjects: %v", err)
	}
	var found bool
	for _, o := range listed.Objects {
		if o.Key == key && o.SoftDeleted {
			found = true
		}
	}
	if !found {
		t.Errorf("soft-deleted object %q not found in listing: %+v", key, listed.Objects)
	}

	if err := cli.RestoreSoftDeletedObject(ctx, bucket, key, ""); err != nil {
		t.Fatalf("RestoreSoftDeletedObject: %v", err)
	}
}
