package handler

import "testing"

func TestExtractLastUserMessageRawContentPreservesArrayContent(t *testing.T) {
	body := []byte(`{"messages":[{"role":"system","content":"system"},{"role":"user","content":[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}]}]}`)

	got := extractLastUserMessageRawContent(body)
	want := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}]`
	if got != want {
		t.Fatalf("raw content mismatch\nwant: %s\n got: %s", want, got)
	}

	if text := extractLastUserMessage(body); text != "Describe this" {
		t.Fatalf("text summary mismatch: %q", text)
	}
}
