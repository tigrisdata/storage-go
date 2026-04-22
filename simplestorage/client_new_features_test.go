package simplestorage

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/joho/godotenv/autoload"
)

func TestProgressReader(t *testing.T) {
	const payload = "hello world"
	var reports []UploadProgress

	pr := &progressReader{
		reader:   strings.NewReader(payload),
		total:    int64(len(payload)),
		callback: func(p UploadProgress) { reports = append(reports, p) },
	}

	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
	if len(reports) == 0 {
		t.Fatal("expected at least one progress report")
	}

	last := reports[len(reports)-1]
	if last.Loaded != int64(len(payload)) {
		t.Errorf("last.Loaded = %d, want %d", last.Loaded, len(payload))
	}
	if last.Total != int64(len(payload)) {
		t.Errorf("last.Total = %d, want %d", last.Total, len(payload))
	}
	if last.Percentage != 100.0 {
		t.Errorf("last.Percentage = %f, want 100.0", last.Percentage)
	}
}

func TestBucketOption_WithBucketAccess(t *testing.T) {
	o := new(BucketOptions).defaults()
	WithBucketAccess(AccessPublic)(&o)

	if o.Access != AccessPublic {
		t.Errorf("WithBucketAccess() set Access = %v, want %v", o.Access, AccessPublic)
	}
}

func TestListResult_CommonPrefixes(t *testing.T) {
	result := &ListResult{
		CommonPrefixes: []string{"reports/2023/", "reports/2024/"},
	}

	if len(result.CommonPrefixes) != 2 {
		t.Errorf("CommonPrefixes length = %d, want 2", len(result.CommonPrefixes))
	}
}

// TestHead tests the Head method for metadata-only object retrieval.
func TestHead(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setupEnv func() func()
		wantErr  bool
	}{
		{
			name: "head requires valid client",
			key:  "test-key",
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: true, // Will fail without real credentials
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv
			defer cleanup()

			client, err := New(context.Background(),
				WithEndpoint("https://test.endpoint.dev"),
			)
			if err != nil && !errors.Is(err, ErrNoBucketName) {
				t.Fatalf("New() failed: %v", err)
			}

			if err == nil {
				_, err = client.Head(context.Background(), tt.key)

				if tt.wantErr && err == nil {
					t.Errorf("Head() expected error, got nil")
				} else if !tt.wantErr && err != nil {
					t.Errorf("Head() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestGetPresignedUrl tests the GetPresignedUrl method.
func TestGetPresignedUrl(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		opts     []ClientOption
		setupEnv func() func()
		wantErr  bool
	}{
		{
			name: "generates presigned URL with default expiration",
			key:  "test-key",
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: false,
		},
		{
			name: "generates presigned URL with custom expiration",
			key:  "test-key",
			opts: []ClientOption{
				WithPresignedExpiresIn(7200),
			},
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: false,
		},
		{
			name: "empty key returns error",
			key:  "",
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: true, // Empty key causes presigner to fail
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
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			result, err := client.GetPresignedUrl(context.Background(), tt.key, tt.opts...)

			if tt.wantErr && err == nil {
				t.Errorf("GetPresignedUrl() expected error, got nil")
			} else if !tt.wantErr {
				if err != nil {
					t.Errorf("GetPresignedUrl() unexpected error = %v", err)
				} else if result == nil {
					t.Errorf("GetPresignedUrl() expected result, got nil")
				} else if result.URL == "" {
					t.Errorf("GetPresignedUrl() expected non-empty URL, got empty string")
				} else if !strings.HasPrefix(result.URL, "https://") {
					t.Errorf("GetPresignedUrl() URL should start with https://, got %s", result.URL)
				}
			}
		})
	}
}

// TestClientOptions tests various ClientOption functions.
func TestClientOptions(t *testing.T) {
	tests := []struct {
		name   string
		option ClientOption
		verify func(*testing.T, *ClientOptions)
	}{
		{
			name:   "WithDelimiter sets Delimiter",
			option: WithDelimiter("/"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.Delimiter == nil || *o.Delimiter != "/" {
					t.Errorf("WithDelimiter() set Delimiter = %v, want %v", o.Delimiter, "/")
				}
			},
		},
		{
			name:   "WithQuerySnapshotVersion sets SnapshotVersion",
			option: WithQuerySnapshotVersion("test-version"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.SnapshotVersion == nil || *o.SnapshotVersion != "test-version" {
					t.Errorf("WithQuerySnapshotVersion() set SnapshotVersion = %v, want %v", o.SnapshotVersion, "test-version")
				}
			},
		},
		{
			name:   "WithResponseContentType sets ResponseContentType",
			option: WithResponseContentType("text/plain"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseContentType == nil || *o.ResponseContentType != "text/plain" {
					t.Errorf("WithResponseContentType() set ResponseContentType = %v, want %v", o.ResponseContentType, "text/plain")
				}
			},
		},
		{
			name:   "WithResponseContentDisposition sets ResponseContentDisposition",
			option: WithResponseContentDisposition("attachment"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseContentDisposition == nil || *o.ResponseContentDisposition != "attachment" {
					t.Errorf("WithResponseContentDisposition() set ResponseContentDisposition = %v, want %v", o.ResponseContentDisposition, "attachment")
				}
			},
		},
		{
			name:   "WithResponseCacheControl sets ResponseCacheControl",
			option: WithResponseCacheControl("no-cache"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseCacheControl == nil || *o.ResponseCacheControl != "no-cache" {
					t.Errorf("WithResponseCacheControl() set ResponseCacheControl = %v, want %v", o.ResponseCacheControl, "no-cache")
				}
			},
		},
		{
			name:   "WithRandomSuffix sets RandomSuffix to true",
			option: WithRandomSuffix(),
			verify: func(t *testing.T, o *ClientOptions) {
				if !o.RandomSuffix {
					t.Errorf("WithRandomSuffix() did not set RandomSuffix to true")
				}
			},
		},
		{
			name:   "WithAllowOverwrite sets AllowOverwrite",
			option: WithAllowOverwrite(false),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.AllowOverwrite == nil || *o.AllowOverwrite != false {
					t.Errorf("WithAllowOverwrite() set AllowOverwrite = %v, want %v", o.AllowOverwrite, false)
				}
			},
		},
		{
			name:   "WithContentDisposition sets ContentDisposition",
			option: WithContentDisposition("attachment; filename=test.txt"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ContentDisposition == nil || *o.ContentDisposition != "attachment; filename=test.txt" {
					t.Errorf("WithContentDisposition() set ContentDisposition = %v, want %v", o.ContentDisposition, "attachment; filename=test.txt")
				}
			},
		},
		{
			name:   "WithMultipartUpload sets MultipartThreshold",
			option: WithMultipartUpload(5242880),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.MultipartThreshold == nil || *o.MultipartThreshold != 5242880 {
					t.Errorf("WithMultipartUpload() set MultipartThreshold = %v, want %v", o.MultipartThreshold, 5242880)
				}
			},
		},
		{
			name:   "WithUploadProgress sets UploadProgressCallback",
			option: WithUploadProgress(func(p UploadProgress) {}),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.UploadProgressCallback == nil {
					t.Errorf("WithUploadProgress() did not set UploadProgressCallback")
				}
			},
		},
		{
			name:   "WithPresignedExpiresIn sets PresignedExpiresIn",
			option: WithPresignedExpiresIn(1800),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.PresignedExpiresIn == nil || *o.PresignedExpiresIn != 1800 {
					t.Errorf("WithPresignedExpiresIn() set PresignedExpiresIn = %v, want %v", o.PresignedExpiresIn, 1800)
				}
			},
		},
		{
			name:   "WithPresignedContentType sets PresignedContentType",
			option: WithPresignedContentType("image/png"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.PresignedContentType == nil || *o.PresignedContentType != "image/png" {
					t.Errorf("WithPresignedContentType() set PresignedContentType = %v, want %v", o.PresignedContentType, "image/png")
				}
			},
		},
		{
			name:   "WithPrefix sets Prefix",
			option: WithPrefix("test-prefix/"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.Prefix == nil || *o.Prefix != "test-prefix/" {
					t.Errorf("WithPrefix() set Prefix = %v, want %v", o.Prefix, "test-prefix/")
				}
			},
		},
		{
			name:   "WithAccessType sets AccessType",
			option: WithAccessType(AccessPublic),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.AccessType != AccessPublic {
					t.Errorf("WithAccessType() set AccessType = %v, want %v", o.AccessType, AccessPublic)
				}
			},
		},
		{
			name:   "WithPresignedOperation sets PresignedOperation",
			option: WithPresignedOperation(PresignOpPut),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.PresignedOperation != PresignOpPut {
					t.Errorf("WithPresignedOperation() set PresignedOperation = %v, want %v", o.PresignedOperation, PresignOpPut)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := new(ClientOptions).defaults(Options{})
			if tt.option != nil {
				tt.option(&o)
			}
			if tt.verify != nil {
				tt.verify(t, &o)
			}
		})
	}
}

// TestList tests the List method with new ListResult return type.
func TestList(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		opts     []ClientOption
		setupEnv func() func()
		wantErr  bool
	}{
		{
			name:   "list with default options",
			prefix: "test-prefix",
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: true, // Will fail without real credentials
		},
		{
			name:   "list with delimiter",
			prefix: "test-prefix",
			opts: []ClientOption{
				WithDelimiter("/"),
			},
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: true,
		},
		{
			name:   "list with prefix option",
			prefix: "",
			opts: []ClientOption{
				WithPrefix("custom-prefix/"),
			},
			setupEnv: func() func() {
				os.Setenv("TIGRIS_STORAGE_BUCKET", "test-bucket")
				return func() { os.Unsetenv("TIGRIS_STORAGE_BUCKET") }
			},
			wantErr: true,
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
			)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			result, err := client.List(context.Background(), tt.prefix, tt.opts...)

			if tt.wantErr && err == nil {
				t.Errorf("List() expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("List() unexpected error = %v", err)
			} else if !tt.wantErr && result == nil {
				t.Errorf("List() expected non-nil result, got nil")
			}
		})
	}
}

// TestPutResponseTypes tests PutResponse fields.
func TestPutResponse(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *PutResponse)
	}{
		{
			name: "PutResponse has all required fields",
			verify: func(t *testing.T, r *PutResponse) {
				if r == nil {
					t.Error("PutResponse is nil")
					return
				}
				// Check that fields are accessible (not checking values since they're runtime-dependent)
				_ = r.Path
				_ = r.Size
				_ = r.Modified
				_ = r.ContentType
				_ = r.ContentDisposition
				_ = r.URL
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &PutResponse{}
			if tt.verify != nil {
				tt.verify(t, resp)
			}
		})
	}
}

// TestHeadResponseTypes tests HeadResponse fields.
func TestHeadResponse(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *HeadResponse)
	}{
		{
			name: "HeadResponse has all required fields",
			verify: func(t *testing.T, r *HeadResponse) {
				if r == nil {
					t.Error("HeadResponse is nil")
					return
				}
				// Check that fields are accessible (not checking values since they're runtime-dependent)
				_ = r.Path
				_ = r.Size
				_ = r.Modified
				_ = r.ContentType
				_ = r.ContentDisposition
				_ = r.URL
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &HeadResponse{}
			if tt.verify != nil {
				tt.verify(t, resp)
			}
		})
	}
}

// TestListResultTypes tests ListResult fields.
func TestListResult(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *ListResult)
	}{
		{
			name: "ListResult has all required fields",
			verify: func(t *testing.T, r *ListResult) {
				if r == nil {
					t.Error("ListResult is nil")
					return
				}
				// Check that fields are accessible
				if r.Items == nil {
					t.Error("Items is nil (should be empty slice, not nil)")
				}
				_ = r.PaginationToken
				_ = r.HasMore
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &ListResult{Items: []Object{}}
			if tt.verify != nil {
				tt.verify(t, resp)
			}
		})
	}
}

// TestObjectListItem tests Object fields when used in List results.
func TestObjectListItem(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *Object)
	}{
		{
			name: "Object has all required fields for List results",
			verify: func(t *testing.T, i *Object) {
				if i == nil {
					t.Error("Object is nil")
					return
				}
				_ = i.Bucket
				_ = i.Key
				_ = i.Etag
				_ = i.Size
				_ = i.LastModified
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &Object{}
			if tt.verify != nil {
				tt.verify(t, item)
			}
		})
	}
}

// TestGetPresignedUrlResultTypes tests GetPresignedUrlResult fields.
func TestGetPresignedUrlResult(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *GetPresignedUrlResult)
	}{
		{
			name: "GetPresignedUrlResult has all required fields",
			verify: func(t *testing.T, r *GetPresignedUrlResult) {
				if r == nil {
					t.Error("GetPresignedUrlResult is nil")
					return
				}
				_ = r.URL
				_ = r.ExpiresIn
				_ = r.Method
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &GetPresignedUrlResult{}
			if tt.verify != nil {
				tt.verify(t, result)
			}
		})
	}
}

// TestUploadProgressTypes tests UploadProgress fields.
func TestUploadProgress(t *testing.T) {
	tests := []struct {
		name   string
		verify func(*testing.T, *UploadProgress)
	}{
		{
			name: "UploadProgress has all required fields",
			verify: func(t *testing.T, p *UploadProgress) {
				if p == nil {
					t.Error("UploadProgress is nil")
					return
				}
				_ = p.Loaded
				_ = p.Total
				_ = p.Percentage
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := &UploadProgress{}
			if tt.verify != nil {
				tt.verify(t, progress)
			}
		})
	}
}
