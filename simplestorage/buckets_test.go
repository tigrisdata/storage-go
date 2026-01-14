package simplestorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
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
	_, err := client.CreateBucket(ctx, bucket)
	if err != nil {
		t.Fatalf("setupTestBucket: failed to create bucket %s: %v", bucket, err)
	}
	return bucket
}

// cleanupTestBucket deletes a bucket after testing.
// It uses WithForceDelete() to ensure the bucket is deleted even if not empty.
// Logs errors instead of failing, to avoid masking test failures.
func cleanupTestBucket(t *testing.T, ctx context.Context, client *Client, bucket string) {
	t.Helper()
	err := client.DeleteBucket(ctx, bucket, WithForceDelete())
	if err != nil {
		t.Logf("cleanupTestBucket: failed to delete bucket %s: %v", bucket, err)
	}
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
			wantErr:       errors.New("simplestorage: bucket name required for bucket management operations"),
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

			// Create a client (use a dummy bucket for object operations)
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
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
				} else if !strings.Contains(err.Error(), tt.wantErr.Error()) {
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
			wantErr:  errors.New("simplestorage: bucket name required for bucket management operations"),
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

			// Create a client
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
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
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
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

			// Create a client
			os.Setenv("TIGRIS_STORAGE_BUCKET", "dummy-bucket")
			defer os.Unsetenv("TIGRIS_STORAGE_BUCKET")

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			_, err = client.ListBuckets(context.Background())

			if tt.wantErr && err == nil {
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
			name:   "WithForceDelete sets ForceDelete",
			option: WithForceDelete(),
			verify: func(t *testing.T, o *BucketOptions) {
				if !o.ForceDelete {
					t.Errorf("WithForceDelete() did not set ForceDelete")
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

func TestListBucketSnapshots(t *testing.T) {
	tests := []struct {
		name    string
		bucket  string
		wantErr bool
	}{
		{
			name:    "empty bucket name returns error",
			bucket:  "",
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

			_, err = client.ListBucketSnapshots(context.Background(), tt.bucket)

			if tt.wantErr && err == nil {
				t.Errorf("ListBucketSnapshots() expected error, got nil")
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
