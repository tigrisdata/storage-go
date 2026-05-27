package simplestorage

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/tigrisdata/storage-go/tigrisheaders"
)

// skipIfNoCreds skips the test if Tigris credentials are not set.
// Use this for integration tests that require real Tigris operations.
func skipIfNoCreds(t *testing.T) {
	t.Helper()
	if os.Getenv("TIGRIS_STORAGE_ACCESS_KEY_ID") == "" ||
		os.Getenv("TIGRIS_STORAGE_SECRET_ACCESS_KEY") == "" {
		t.Skip("skipping: TIGRIS_STORAGE_ACCESS_KEY_ID and TIGRIS_STORAGE_SECRET_ACCESS_KEY not set")
	}
}

// setupTestBucket creates a bucket for testing and returns its name.
// The caller should use cleanupTestBucket to delete it after the test.
func setupTestBucket(t *testing.T, ctx context.Context, client *Client) string {
	t.Helper()
	skipIfNoCreds(t)

	bucket := fmt.Sprintf("test-bucket-%d", time.Now().UnixNano())
	_, err := client.CreateBucket(ctx, bucket, WithEnableSnapshot())
	if err != nil {
		t.Fatalf("setupTestBucket: failed to create bucket %s: %v", bucket, err)
	}

	t.Cleanup(func() {
		err := client.DeleteBucket(context.Background(), bucket, func(bo *BucketOptions) {
			bo.S3Options = append(bo.S3Options, tigrisheaders.WithHeader("Tigris-Force-Delete", "true"))
		})
		if err != nil {
			t.Logf("cleanupTestBucket: failed to delete bucket %s: %v", bucket, err)
		}
	})

	return bucket
}

func TestCreateBucket(t *testing.T) {
	tests := []struct {
		name          string
		bucket        string
		setupEnv      func() func()
		options       []BucketOption
		wantErr       error
		errCheck      func(error) bool
		skipIfNoCreds bool
	}{
		{
			name:          "empty bucket name returns error",
			bucket:        "",
			wantErr:       ErrBucketNameRequired,
			skipIfNoCreds: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipIfNoCreds {
				skipIfNoCreds(t)
			}

			cleanup := tt.setupEnv
			if cleanup == nil {
				cleanup = func() func() { return func() {} }
			}
			defer cleanup()

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
				WithBucket("xxx-dummy-bucket"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			_, err = client.CreateBucket(context.Background(), tt.bucket, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("CreateBucket() expected error, got nil")
					return
				}
				if tt.errCheck != nil {
					if !tt.errCheck(err) {
						t.Errorf("CreateBucket() error = %v, want error matching %v", err, tt.wantErr)
					}
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("CreateBucket() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("CreateBucket() unexpected error = %v", err)
			}
		})
	}
}

func TestDeleteBucket(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		setupEnv func() func()
		options  []BucketOption
		wantErr  error
	}{
		{
			name:     "empty bucket name returns error",
			bucket:   "",
			wantErr:  ErrBucketNameRequired,
			setupEnv: func() func() { return func() {} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv
			if cleanup == nil {
				cleanup = func() func() { return func() {} }
			}
			defer cleanup()

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
				WithBucket("xxx-dummy-bucket"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			err = client.DeleteBucket(context.Background(), tt.bucket, tt.options...)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("DeleteBucket() expected error, got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("DeleteBucket() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("DeleteBucket() unexpected error = %v", err)
			}
		})
	}
}

func TestListBuckets(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func() func()
		wantErr  bool
	}{
		{
			name: "list buckets requires credentials",
			setupEnv: func() func() {
				// Ensure no credentials are set
				os.Unsetenv("TIGRIS_STORAGE_ACCESS_KEY_ID")
				os.Unsetenv("TIGRIS_STORAGE_SECRET_ACCESS_KEY")
				return func() {}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv
			defer cleanup()

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
				WithBucket("xxx-dummy-bucket"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			var iterErr error
			for _, err := range client.Buckets(context.Background()) {
				if err != nil {
					iterErr = err
					break
				}
			}

			if tt.wantErr && iterErr == nil {
				t.Errorf("ListBuckets() expected error, got nil")
			}
		})
	}
}

func TestGetBucketInfo(t *testing.T) {
	tests := []struct {
		name     string
		bucket   string
		setupEnv func() func()
		wantErr  bool
	}{
		{
			name:     "empty bucket name returns error",
			bucket:   "",
			wantErr:  true,
			setupEnv: func() func() { return func() {} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv
			defer cleanup()

			// Create a client
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			_, err = client.GetBucketInfo(context.Background(), tt.bucket)

			if tt.wantErr && err == nil {
				t.Errorf("GetBucketInfo() expected error, got nil")
			}
		})
	}
}

func TestBucketOptions(t *testing.T) {
	tests := []struct {
		name   string
		option BucketOption
		verify func(*testing.T, *BucketOptions)
	}{
		{
			name:   "WithEnableSnapshot sets EnableSnapshot",
			option: WithEnableSnapshot(),
			verify: func(t *testing.T, o *BucketOptions) {
				if !o.EnableSnapshot {
					t.Errorf("WithEnableSnapshot() did not set EnableSnapshot")
				}
			},
		},
		{
			name:   "WithSnapshotVersion sets SnapshotVersion",
			option: WithSnapshotVersion("test-version"),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.SnapshotVersion != "test-version" {
					t.Errorf("WithSnapshotVersion() set version = %v, want %v", o.SnapshotVersion, "test-version")
				}
			},
		},
		{
			name:   "WithBucketRegion sets Region",
			option: WithBucketRegion("fra"),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.Region != "fra" {
					t.Errorf("WithBucketRegion() set region = %v, want %v", o.Region, "fra")
				}
			},
		},
		{
			name:   "WithListLimit sets MaxKeys",
			option: WithListLimit(100),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.MaxKeys == nil || *o.MaxKeys != 100 {
					t.Errorf("WithListLimit() set MaxKeys = %v, want %v", o.MaxKeys, 100)
				}
			},
		},
		{
			name:   "WithListToken sets ContinuationToken",
			option: WithListToken("test-token"),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.ContinuationToken == nil || *o.ContinuationToken != "test-token" {
					t.Errorf("WithListToken() set ContinuationToken = %v, want %v", o.ContinuationToken, "test-token")
				}
			},
		},
		{
			name:   "WithDefaultTier sets DefaultTier",
			option: WithDefaultTier("STANDARD_IA"),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.DefaultTier != "STANDARD_IA" {
					t.Errorf("WithDefaultTier() set DefaultTier = %v, want %v", o.DefaultTier, "STANDARD_IA")
				}
			},
		},
		{
			name:   "WithConsistentRead sets Consistency",
			option: WithConsistentRead(),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.Consistency != "strict" {
					t.Errorf("WithConsistentRead() set Consistency = %v, want %v", o.Consistency, "strict")
				}
			},
		},
		{
			name:   "WithForkSourceSnapshot sets SourceBucketSnapshot",
			option: WithForkSourceSnapshot("snapshot-123"),
			verify: func(t *testing.T, o *BucketOptions) {
				if o.SourceBucketSnapshot != "snapshot-123" {
					t.Errorf("WithForkSourceSnapshot() set SourceBucketSnapshot = %v, want %v", o.SourceBucketSnapshot, "snapshot-123")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := new(BucketOptions).defaults()
			if tt.option != nil {
				tt.option(&o)
			}
			if tt.verify != nil {
				tt.verify(t, &o)
			}
		})
	}
}

func TestCreateBucketSnapshot(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		description string
		wantErr     bool
	}{
		{
			name:        "empty bucket name returns error",
			bucket:      "",
			description: "test snapshot",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a client
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			_, err = client.CreateBucketSnapshot(context.Background(), tt.bucket, tt.description)

			if tt.wantErr && err == nil {
				t.Errorf("CreateBucketSnapshot() expected error, got nil")
			}
		})
	}
}

func TestForkBucket(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		target  string
		wantErr bool
	}{
		{
			name:    "empty source bucket name returns error",
			source:  "",
			target:  "target-bucket",
			wantErr: true,
		},
		{
			name:    "empty target bucket name returns error",
			source:  "source-bucket",
			target:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a client
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			_, err = client.ForkBucket(context.Background(), tt.source, tt.target)

			if tt.wantErr && err == nil {
				t.Errorf("ForkBucket() expected error, got nil")
			}
		})
	}
}

// TestBucketLifecycle_integration tests the full bucket lifecycle with real Tigris operations.
// This test requires TIGRIS_STORAGE_ACCESS_KEY_ID and TIGRIS_STORAGE_SECRET_ACCESS_KEY to be set.
func TestBucketLifecycle_integration(t *testing.T) {
	skipIfNoCreds(t)

	ctx := context.Background()
	os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
	defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

	client, err := New(ctx)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Use setupTestBucket to verify the helper works
	bucket := setupTestBucket(t, ctx, client)

	// Verify bucket was created
	info, err := client.GetBucketInfo(ctx, bucket)
	if err != nil {
		t.Errorf("GetBucketInfo() failed: %v", err)
	}
	if info.Name != bucket {
		t.Errorf("GetBucketInfo() returned bucket name %s, want %s", info.Name, bucket)
	}
}

func TestBucketSnapshotList(t *testing.T) {
	skipIfNoCreds(t)
	ctx := t.Context()

	client, err := New(ctx, WithBucket("xxx-foo-test"))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	bucket := setupTestBucket(t, ctx, client)
	client = client.For(bucket)

	snapshotName := t.Name()
	sn, err := client.CreateBucketSnapshot(ctx, bucket, snapshotName)
	if err != nil {
		t.Fatalf("CreateBucketSnapshot(%q, %q) failed: %v", bucket, snapshotName, err)
	}

	snaps, err := collect(client.Snapshots(ctx, bucket))
	if err != nil {
		t.Fatalf("ListBucketSnapshots(%q) failed: %v", bucket, err)
	}

	if len(snaps) != 1 {
		t.Errorf("wanted len(snaps) == 1 but got: %d", len(snaps))
	}

	gotSN := snaps[0]

	if gotSN.Version != sn.Version {
		t.Errorf("wanted snapshot version %s but got: %s", sn.Version, gotSN.Version)
	}

	if gotSN.Name != sn.Name {
		t.Errorf("wanted snapshot name %q but got: %q", sn.Name, gotSN.Name)
	}
}

func collect[T any](i iter.Seq2[T, error]) ([]T, error) {
	var result []T
	var errs []error

	for item, err := range i {
		if err != nil {
			errs = append(errs, err)
		}

		result = append(result, item)
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return result, nil
}
