package httputil

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestTransitAICompressedBodyLimitKeepsMemoryBudget(t *testing.T) {
	for _, encoding := range []string{"gzip", "zstd"} {
		t.Run(encoding, func(t *testing.T) {
			payload := bytes.Repeat([]byte{'x'}, 1024)
			var compressed []byte
			if encoding == "gzip" {
				var buffer bytes.Buffer
				writer := gzip.NewWriter(&buffer)
				if _, err := writer.Write(payload); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				compressed = buffer.Bytes()
			} else {
				writer, err := zstd.NewWriter(nil)
				if err != nil {
					t.Fatal(err)
				}
				compressed = writer.EncodeAll(payload, nil)
				_ = writer.Close()
			}
			for _, limit := range []int64{512, 1024} {
				budget := NewBodyMemoryBudget(16 << 20)
				ctx, release := WithRequestBodyMemoryBudget(context.Background(), budget)
				defer release()
				ctx = WithRequestBodyLimit(ctx, limit)
				req := newRequestWithBody(t, compressed, encoding).WithContext(ctx)
				body, err := ReadRequestBodyWithPrealloc(req)
				if limit < int64(len(payload)) {
					var maxErr *http.MaxBytesError
					if body != nil || !errors.As(err, &maxErr) || maxErr.Limit != limit || !errors.Is(err, ErrDecompressedBodyTooLarge) {
						t.Fatalf("%s limit=%d: len=%d err=%v", encoding, limit, len(body), err)
					}
					if used := requestBodyBudgetUsed(budget); used != 0 {
						t.Fatalf("failed decode retained %d bytes", used)
					}
				} else {
					if err != nil || !bytes.Equal(body, payload) {
						t.Fatalf("%s exact limit: len=%d err=%v", encoding, len(body), err)
					}
					if used := requestBodyBudgetUsed(budget); used < int64(len(body)) {
						t.Fatalf("final body escaped memory budget: %d", used)
					}
				}
				release()
				if used := requestBodyBudgetUsed(budget); used != 0 {
					t.Fatalf("cleanup retained %d bytes", used)
				}
			}
		})
	}
}

func TestTransitAIConfiguredBodyLimitAcceptsLargeCompressedInput(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	chunk := bytes.Repeat([]byte{'x'}, 1<<20)
	for range 65 {
		if _, err := writer.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	budget := NewBodyMemoryBudget(192 << 20)
	ctx, release := WithRequestBodyMemoryBudget(context.Background(), budget)
	defer release()
	ctx = WithRequestBodyLimit(ctx, 500<<20)
	req := newRequestWithBody(t, compressed.Bytes(), "gzip").WithContext(ctx)
	body, err := ReadRequestBodyWithPrealloc(req)
	if err != nil || len(body) != 65<<20 {
		t.Fatalf("configured large-body support regressed: len=%d err=%v", len(body), err)
	}
	if body[0] != 'x' || body[len(body)-1] != 'x' {
		t.Fatal("decoded content changed")
	}
	release()
	if used := requestBodyBudgetUsed(budget); used != 0 {
		t.Fatalf("cleanup retained %d bytes", used)
	}
}
