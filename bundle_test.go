package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBundleObjects_Validation(t *testing.T) {
	cli := &Client{}

	t.Run("missing bucket", func(t *testing.T) {
		_, err := cli.BundleObjects(context.Background(), &BundleObjectsInput{
			Keys: []string{"a"},
		})
		if err == nil {
			t.Fatal("expected error for missing bucket")
		}
	})

	t.Run("missing keys", func(t *testing.T) {
		_, err := cli.BundleObjects(context.Background(), &BundleObjectsInput{
			Bucket: "test-bucket",
		})
		if err == nil {
			t.Fatal("expected error for missing keys")
		}
	})

	t.Run("empty keys", func(t *testing.T) {
		_, err := cli.BundleObjects(context.Background(), &BundleObjectsInput{
			Bucket: "test-bucket",
			Keys:   []string{},
		})
		if err == nil {
			t.Fatal("expected error for empty keys")
		}
	})
}

func TestBundleObjects_RequestConstruction(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/x-tar")
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

	t.Run("default options", func(t *testing.T) {
		output, err := cli.BundleObjects(context.Background(), &BundleObjectsInput{
			Bucket: "my-bucket",
			Keys:   []string{"a.jpg", "b.jpg"},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer output.Body.Close()

		if capturedReq.Method != "POST" {
			t.Errorf("method = %q, want POST", capturedReq.Method)
		}
		if capturedReq.URL.Path != "/my-bucket" {
			t.Errorf("path = %q, want /my-bucket", capturedReq.URL.Path)
		}
		if capturedReq.URL.Query().Get("bundle") != "" {
			// query param "bundle" should be present with empty value
		} else if !capturedReq.URL.Query().Has("bundle") {
			t.Error("missing ?bundle query parameter")
		}

		if capturedReq.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q, want application/json", capturedReq.Header.Get("Content-Type"))
		}
		if capturedReq.Header.Get("X-Tigris-Bundle-Format") != "tar" {
			t.Errorf("bundle-format = %q, want tar", capturedReq.Header.Get("X-Tigris-Bundle-Format"))
		}
		if capturedReq.Header.Get("X-Tigris-Bundle-Compression") != "none" {
			t.Errorf("compression = %q, want none", capturedReq.Header.Get("X-Tigris-Bundle-Compression"))
		}
		if capturedReq.Header.Get("X-Tigris-Bundle-On-Error") != "skip" {
			t.Errorf("on-error = %q, want skip", capturedReq.Header.Get("X-Tigris-Bundle-On-Error"))
		}

		// Verify body contains keys.
		var body bundleRequestBody
		if err := json.Unmarshal(capturedBody, &body); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if len(body.Keys) != 2 || body.Keys[0] != "a.jpg" || body.Keys[1] != "b.jpg" {
			t.Errorf("body keys = %v, want [a.jpg b.jpg]", body.Keys)
		}

		// Verify SigV4 authorization header is present.
		if capturedReq.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header (SigV4)")
		}

		if output.ContentType != "application/x-tar" {
			t.Errorf("content-type = %q, want application/x-tar", output.ContentType)
		}
	})

	t.Run("custom compression and error mode", func(t *testing.T) {
		output, err := cli.BundleObjects(context.Background(), &BundleObjectsInput{
			Bucket:      "my-bucket",
			Keys:        []string{"x.txt"},
			Compression: BundleCompressionGzip,
			OnError:     BundleOnErrorFail,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer output.Body.Close()

		if capturedReq.Header.Get("X-Tigris-Bundle-Compression") != "gzip" {
			t.Errorf("compression = %q, want gzip", capturedReq.Header.Get("X-Tigris-Bundle-Compression"))
		}
		if capturedReq.Header.Get("X-Tigris-Bundle-On-Error") != "fail" {
			t.Errorf("on-error = %q, want fail", capturedReq.Header.Get("X-Tigris-Bundle-On-Error"))
		}
	})
}

func TestBundleObjects_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<Error><Code>InvalidArgument</Code></Error>`))
	}))
	defer server.Close()

	cli, err := New(context.Background(),
		WithEndpoint(server.URL),
		WithAccessKeypair("test-key", "test-secret"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cli.BundleObjects(context.Background(), &BundleObjectsInput{
		Bucket: "my-bucket",
		Keys:   []string{"a.jpg"},
	})
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
}
