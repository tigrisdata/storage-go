package simplestorage

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestListOptions(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		option ListOption
		verify func(*testing.T, *listOptions)
	}{
		{
			name:   "WithContinueToken sets ContinueToken",
			option: WithContinueToken("next-page"),
			verify: func(t *testing.T, lo *listOptions) {
				if lo.ContinueToken == nil || *lo.ContinueToken != "next-page" {
					t.Errorf("ContinueToken = %v, want %q", lo.ContinueToken, "next-page")
				}
			},
		},
		{
			name:   "WithDelimiter sets Delimiter",
			option: WithDelimiter("/"),
			verify: func(t *testing.T, lo *listOptions) {
				if lo.Delimiter == nil || *lo.Delimiter != "/" {
					t.Errorf("Delimiter = %v, want %q", lo.Delimiter, "/")
				}
			},
		},
		{
			name:   "WithMaxKeys sets MaxKeys",
			option: WithMaxKeys(250),
			verify: func(t *testing.T, lo *listOptions) {
				if lo.MaxKeys == nil || *lo.MaxKeys != 250 {
					t.Errorf("MaxKeys = %v, want 250", lo.MaxKeys)
				}
			},
		},
		{
			name:   "WithPrefix sets Prefix",
			option: WithPrefix("test-prefix/"),
			verify: func(t *testing.T, lo *listOptions) {
				if lo.Prefix == nil || *lo.Prefix != "test-prefix/" {
					t.Errorf("Prefix = %v, want %q", lo.Prefix, "test-prefix/")
				}
			},
		},
		{
			name:   "WithStartAfter sets StartAfter",
			option: WithStartAfter("key-42"),
			verify: func(t *testing.T, lo *listOptions) {
				if lo.StartAfter == nil || *lo.StartAfter != "key-42" {
					t.Errorf("StartAfter = %v, want %q", lo.StartAfter, "key-42")
				}
			},
		},
		{
			name: "WithListS3Options appends to S3Options",
			option: WithListS3Options(
				func(*s3.Options) {},
				func(*s3.Options) {},
			),
			verify: func(t *testing.T, lo *listOptions) {
				if len(lo.S3Options) != 2 {
					t.Errorf("len(S3Options) = %d, want 2", len(lo.S3Options))
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var lo listOptions
			tt.option(&lo)
			tt.verify(t, &lo)
		})
	}
}
