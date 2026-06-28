package handler

import "testing"

func TestExtractLastUserMessageRawContentPreservesArrayContent(t *testing.T) {
	body := []byte(`{"messages":[{"role":"system","content":"system"},{"role":"user","content":[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/image.png"}}]}]}`)

	got := extractLastUserMessageRawContent(body)
	want := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/image.png"}}]`
	if got != want {
		t.Fatalf("raw content mismatch\nwant: %s\n got: %s", want, got)
	}

	if text := extractLastUserMessage(body); text != "Describe this" {
		t.Fatalf("text summary mismatch: %q", text)
	}
}

func TestUserAIRequestHasDataImageURL(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}]}]}`)
	if !userAIRequestHasDataImageURL(body) {
		t.Fatal("expected data image URL to be detected")
	}

	body = []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/image.png"}}]}]}`)
	if userAIRequestHasDataImageURL(body) {
		t.Fatal("uploaded image URL should be allowed")
	}
}

func TestRewriteUserAIRelativeImageURLs(t *testing.T) {
	payload := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "/uploads/user_ai/7/image.jpg"}},
					map[string]any{"type": "image_url", "image_url": "https://cdn.example/image.jpg"},
				},
			},
		},
	}

	rewriteUserAIRelativeImageURLs(payload, "https://chat.example")
	messages := payload["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	first := content[0].(map[string]any)["image_url"].(map[string]any)["url"]
	if first != "https://chat.example/uploads/user_ai/7/image.jpg" {
		t.Fatalf("relative image URL not rewritten: %v", first)
	}
	second := content[1].(map[string]any)["image_url"]
	if second != "https://cdn.example/image.jpg" {
		t.Fatalf("absolute image URL should not be rewritten: %v", second)
	}
}

func TestParseOptionalBoolValue(t *testing.T) {
	tests := map[any]bool{
		true:       true,
		false:      false,
		"true":     true,
		" TRUE ":   true,
		"1":        true,
		"yes":      true,
		"on":       true,
		"false":    false,
		"0":        false,
		"":         false,
		float64(1): false,
	}

	for input, want := range tests {
		if got := parseOptionalBoolValue(input); got != want {
			t.Fatalf("parseOptionalBoolValue(%v) = %v, want %v", input, got, want)
		}
	}
}

func TestDeleteUserAIEphemeralFields(t *testing.T) {
	payload := map[string]any{
		"user_ai_ephemeral": true,
		"metadata": map[string]any{
			"user_ai_ephemeral": true,
			"trace_id":          "abc",
		},
	}

	deleteUserAIEphemeralFields(payload)

	if _, ok := payload["user_ai_ephemeral"]; ok {
		t.Fatal("top-level user_ai_ephemeral should be removed")
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should remain when it contains other keys")
	}
	if _, ok := metadata["user_ai_ephemeral"]; ok {
		t.Fatal("metadata user_ai_ephemeral should be removed")
	}
	if metadata["trace_id"] != "abc" {
		t.Fatalf("metadata trace_id should be preserved, got %#v", metadata["trace_id"])
	}

	emptyMetadataPayload := map[string]any{
		"metadata": map[string]any{"user_ai_ephemeral": true},
	}
	deleteUserAIEphemeralFields(emptyMetadataPayload)
	if _, ok := emptyMetadataPayload["metadata"]; ok {
		t.Fatal("empty metadata should be removed")
	}
}
