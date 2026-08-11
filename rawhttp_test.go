package storage

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
)

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		header        http.Header
		body          string
		wantCode      string
		wantMessage   string
		wantRequestID string
		wantFault     smithy.ErrorFault
	}{
		{
			name:        "S3 error document",
			statusCode:  http.StatusNotFound,
			body:        `<Error><Code>NoSuchBucket</Code><Message>The specified bucket does not exist</Message></Error>`,
			wantCode:    "NoSuchBucket",
			wantMessage: "The specified bucket does not exist",
			wantFault:   smithy.FaultClient,
		},
		{
			name:          "request ID from the body wins over the header",
			statusCode:    http.StatusForbidden,
			header:        http.Header{"X-Amz-Request-Id": []string{"header-id"}},
			body:          `<Error><Code>AccessDenied</Code><RequestId>body-id</RequestId></Error>`,
			wantCode:      "AccessDenied",
			wantRequestID: "body-id",
			wantFault:     smithy.FaultClient,
		},
		{
			name:          "request ID from the header when the body has none",
			statusCode:    http.StatusForbidden,
			header:        http.Header{"X-Amz-Request-Id": []string{"header-id"}},
			body:          `<Error><Code>AccessDenied</Code></Error>`,
			wantCode:      "AccessDenied",
			wantRequestID: "header-id",
			wantFault:     smithy.FaultClient,
		},
		{
			name:        "body that is not an S3 error document",
			statusCode:  http.StatusBadGateway,
			body:        "  502 Bad Gateway\n",
			wantMessage: "502 Bad Gateway",
			wantFault:   smithy.FaultServer,
		},
		{
			name:       "empty body",
			statusCode: http.StatusInternalServerError,
			wantFault:  smithy.FaultServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     tt.header,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			err := httpError(resp, "RestoreBucket")

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v, want *APIError", err)
			}
			if apiErr.Op != "RestoreBucket" {
				t.Errorf("Op = %q, want %q", apiErr.Op, "RestoreBucket")
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.statusCode)
			}
			if apiErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", apiErr.Code, tt.wantCode)
			}
			if apiErr.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
			if apiErr.RequestID != tt.wantRequestID {
				t.Errorf("RequestID = %q, want %q", apiErr.RequestID, tt.wantRequestID)
			}
			if got := apiErr.ErrorFault(); got != tt.wantFault {
				t.Errorf("ErrorFault() = %v, want %v", got, tt.wantFault)
			}
			if got := apiErr.HTTPStatusCode(); got != tt.statusCode {
				t.Errorf("HTTPStatusCode() = %d, want %d", got, tt.statusCode)
			}

			// The whole point of the typed error: errors.As reads it through the
			// same interface it uses for errors from the embedded *s3.Client.
			var smithyErr smithy.APIError
			if !errors.As(err, &smithyErr) {
				t.Fatalf("error = %v, want smithy.APIError", err)
			}
			if smithyErr.ErrorCode() != tt.wantCode {
				t.Errorf("ErrorCode() = %q, want %q", smithyErr.ErrorCode(), tt.wantCode)
			}
			if smithyErr.ErrorMessage() != tt.wantMessage {
				t.Errorf("ErrorMessage() = %q, want %q", smithyErr.ErrorMessage(), tt.wantMessage)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "code and message",
			err:  &APIError{Op: "RestoreBucket", StatusCode: 404, Code: "NoSuchBucket", Message: "not found"},
			want: "storage: RestoreBucket: HTTP 404: NoSuchBucket: not found",
		},
		{
			name: "code only",
			err:  &APIError{Op: "RestoreBucket", StatusCode: 403, Code: "AccessDenied"},
			want: "storage: RestoreBucket: HTTP 403: AccessDenied",
		},
		{
			name: "message only",
			err:  &APIError{Op: "BundleObjects", StatusCode: 502, Message: "502 Bad Gateway"},
			want: "storage: BundleObjects: HTTP 502: 502 Bad Gateway",
		},
		{
			name: "status only",
			err:  &APIError{Op: "SetBucketSoftDelete", StatusCode: 500},
			want: "storage: SetBucketSoftDelete: HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A 1xx, 2xx, or 3xx status never reaches httpError, but the fault must still
// report FaultUnknown rather than blaming a side at random.
func TestAPIError_ErrorFaultUnknown(t *testing.T) {
	err := &APIError{Op: "RestoreBucket", StatusCode: 302}
	if got := err.ErrorFault(); got != smithy.FaultUnknown {
		t.Errorf("ErrorFault() = %v, want %v", got, smithy.FaultUnknown)
	}
}
