package handler

import "testing"

func TestNormalizeUserAIImageSize(t *testing.T) {
	tests := map[string]string{
		"":          "1024x1024",
		"1:1":       "1024x1024",
		"16:9":      "2048x1152",
		"9:16":      "1152x2048",
		"1024x1024": "1024x1024",
	}

	for input, want := range tests {
		if got := normalizeUserAIImageSize(input); got != want {
			t.Fatalf("normalizeUserAIImageSize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExtractUserAIImageResults(t *testing.T) {
	body := []byte(`{"data":[{"url":"https://example.com/a.png"},{"b64_json":"Zm9v"}]}`)
	got := extractUserAIImageResults(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 images, got %d", len(got))
	}
	if got[0] != "https://example.com/a.png" {
		t.Fatalf("unexpected first image: %q", got[0])
	}
	if got[1] != "data:image/png;base64,Zm9v" {
		t.Fatalf("unexpected second image: %q", got[1])
	}
}

func TestDecodeUserAIImageDataURL(t *testing.T) {
	contentType, data, ok := decodeUserAIImageDataURL("data:image/png;base64,Zm9v")
	if !ok {
		t.Fatal("expected data url to decode")
	}
	if contentType != "image/png" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if string(data) != "foo" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
}
