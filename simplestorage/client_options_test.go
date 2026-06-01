package simplestorage

import (
	"io"
	"strings"
	"testing"
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

func TestClientOptions(t *testing.T) {
	tests := []struct {
		name   string
		option ClientOption
		verify func(*testing.T, *ClientOptions)
	}{
		{
			name:   "WithQuerySnapshotVersion sets SnapshotVersion",
			option: WithQuerySnapshotVersion("test-version"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.SnapshotVersion == nil || *o.SnapshotVersion != "test-version" {
					t.Errorf("SnapshotVersion = %v, want %q", o.SnapshotVersion, "test-version")
				}
			},
		},
		{
			name:   "WithResponseContentType sets ResponseContentType",
			option: WithResponseContentType("text/plain"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseContentType == nil || *o.ResponseContentType != "text/plain" {
					t.Errorf("ResponseContentType = %v, want %q", o.ResponseContentType, "text/plain")
				}
			},
		},
		{
			name:   "WithResponseContentDisposition sets ResponseContentDisposition",
			option: WithResponseContentDisposition("attachment"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseContentDisposition == nil || *o.ResponseContentDisposition != "attachment" {
					t.Errorf("ResponseContentDisposition = %v, want %q", o.ResponseContentDisposition, "attachment")
				}
			},
		},
		{
			name:   "WithResponseCacheControl sets ResponseCacheControl",
			option: WithResponseCacheControl("no-cache"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ResponseCacheControl == nil || *o.ResponseCacheControl != "no-cache" {
					t.Errorf("ResponseCacheControl = %v, want %q", o.ResponseCacheControl, "no-cache")
				}
			},
		},
		{
			name:   "WithRandomSuffix sets RandomSuffix to true",
			option: WithRandomSuffix(),
			verify: func(t *testing.T, o *ClientOptions) {
				if !o.RandomSuffix {
					t.Errorf("RandomSuffix = false, want true")
				}
			},
		},
		{
			name:   "WithAllowOverwrite sets AllowOverwrite",
			option: WithAllowOverwrite(false),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.AllowOverwrite == nil || *o.AllowOverwrite != false {
					t.Errorf("AllowOverwrite = %v, want false", o.AllowOverwrite)
				}
			},
		},
		{
			name:   "WithContentDisposition sets ContentDisposition",
			option: WithContentDisposition("attachment; filename=test.txt"),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.ContentDisposition == nil || *o.ContentDisposition != "attachment; filename=test.txt" {
					t.Errorf("ContentDisposition = %v, want %q", o.ContentDisposition, "attachment; filename=test.txt")
				}
			},
		},
		{
			name:   "WithMultipartUpload sets MultipartThreshold",
			option: WithMultipartUpload(5242880),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.MultipartThreshold == nil || *o.MultipartThreshold != 5242880 {
					t.Errorf("MultipartThreshold = %v, want 5242880", o.MultipartThreshold)
				}
			},
		},
		{
			name:   "WithUploadProgress sets UploadProgressCallback",
			option: WithUploadProgress(func(p UploadProgress) {}),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.UploadProgressCallback == nil {
					t.Errorf("UploadProgressCallback = nil, want non-nil")
				}
			},
		},
		{
			name:   "WithAccessType sets AccessType",
			option: WithAccessType(AccessPublic),
			verify: func(t *testing.T, o *ClientOptions) {
				if o.AccessType != AccessPublic {
					t.Errorf("AccessType = %v, want %v", o.AccessType, AccessPublic)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := new(ClientOptions).defaults(Options{})
			tt.option(&o)
			tt.verify(t, &o)
		})
	}
}

func TestBucketOption_WithBucketAccess(t *testing.T) {
	o := new(BucketOptions).defaults()
	WithBucketAccess(AccessPublic)(&o)

	if o.Access != AccessPublic {
		t.Errorf("Access = %v, want %v", o.Access, AccessPublic)
	}
}

func TestBucketOptionsDefaultsAccessIsPrivate(t *testing.T) {
	o := new(BucketOptions).defaults()
	if o.Access != AccessPrivate {
		t.Errorf("default Access = %q, want %q", o.Access, AccessPrivate)
	}
}
