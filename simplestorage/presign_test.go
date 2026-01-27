package simplestorage

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPresignURL(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		key         string
		expiry      time.Duration
		opts        []ClientOption
		wantErr     bool
		errContains string
	}{
		{
			name:   "GET method succeeds",
			method: http.MethodGet,
			key:    "test/file.txt",
			expiry: 15 * time.Minute,
		},
		{
			name:   "PUT method succeeds",
			method: http.MethodPut,
			key:    "test/upload.txt",
			expiry: 15 * time.Minute,
		},
		{
			name:   "DELETE method succeeds",
			method: http.MethodDelete,
			key:    "test/delete.txt",
			expiry: 15 * time.Minute,
		},
		{
			name:        "unsupported method fails",
			method:      "POST",
			key:         "test/file.txt",
			expiry:      15 * time.Minute,
			wantErr:     true,
			errContains: "unsupported HTTP method",
		},
		{
			name:        "empty key fails",
			method:      http.MethodGet,
			key:         "",
			expiry:      15 * time.Minute,
			wantErr:     true,
			errContains: "key cannot be empty",
		},
		{
			name:        "non-positive expiry fails",
			method:      http.MethodGet,
			key:         "test/file.txt",
			expiry:      0,
			wantErr:     true,
			errContains: "invalid expiry duration",
		},
		{
			name:   "PUT with ContentType",
			method: http.MethodPut,
			key:    "test/image.png",
			expiry: 15 * time.Minute,
			opts:   []ClientOption{WithContentType("image/png")},
		},
		{
			name:   "PUT with ContentDisposition",
			method: http.MethodPut,
			key:    "test/file.txt",
			expiry: 15 * time.Minute,
			opts:   []ClientOption{WithContentDisposition("attachment")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				// Test validation logic
				cli := &Client{options: Options{BucketName: "test-bucket"}}
				_, err := cli.PresignURL(context.Background(), tt.method, tt.key, tt.expiry, tt.opts...)
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			// Success cases - integration tests requiring real Tigris bucket
			t.Skip("integration test - requires real Tigris bucket")
		})
	}
}
