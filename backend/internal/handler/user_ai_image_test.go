package handler

import (
	"strings"
	"testing"
)

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

func TestRewriteUserAIImageResponseURLs(t *testing.T) {
	body := []byte(`{"created":1,"data":[{"url":"https://upstream.example/a.png"},{"url":"https://upstream.example/b.png"}]}`)
	rewritten, ok := rewriteUserAIImageResponseURLs(body, []string{
		"/uploads/user_ai/7/generated/a.png",
		"/uploads/user_ai/7/generated/b.png",
	})
	if !ok {
		t.Fatal("expected response body to be rewritten")
	}
	got := string(rewritten)
	if got == string(body) {
		t.Fatal("expected rewritten response to differ from original")
	}
	if !containsAll(got,
		`"/uploads/user_ai/7/generated/a.png"`,
		`"/uploads/user_ai/7/generated/b.png"`,
	) {
		t.Fatalf("rewritten response missing stored urls: %s", got)
	}
}

func TestUserAIImageAssistantContent(t *testing.T) {
	got := userAIImageAssistantContent([]string{
		"/uploads/user_ai/7/generated/a.png",
		"/uploads/user_ai/7/generated/b.png",
	})
	want := `[{"image_url":{"url":"/uploads/user_ai/7/generated/a.png"},"type":"image_url"},{"image_url":{"url":"/uploads/user_ai/7/generated/b.png"},"type":"image_url"}]`
	if got != want {
		t.Fatalf("assistant content mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestParseUserAIGroupRequest(t *testing.T) {
	tests := []struct {
		name      string
		groupID   any
		groupName any
		group     any
		wantID    int64
		wantName  string
	}{
		{name: "numeric group_id", groupID: float64(12), wantID: 12},
		{name: "string group_id", groupID: "12", wantID: 12},
		{name: "group_name", groupName: " chatgpt-plus ", wantName: "chatgpt-plus"},
		{name: "numeric group alias", group: "12", wantID: 12},
		{name: "name group alias", group: " chatgpt-plus ", wantName: "chatgpt-plus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUserAIGroupRequest(tt.groupID, tt.groupName, tt.group)
			if tt.wantID > 0 {
				if got.GroupID == nil || *got.GroupID != tt.wantID {
					t.Fatalf("expected group id %d, got %#v", tt.wantID, got.GroupID)
				}
				return
			}
			if got.GroupName != tt.wantName {
				t.Fatalf("expected group name %q, got %q", tt.wantName, got.GroupName)
			}
		})
	}
}

func TestIsBlockedUserAIImageHost(t *testing.T) {
	if !isBlockedUserAIImageHost("127.0.0.1") {
		t.Fatal("expected localhost ip to be blocked")
	}
	if isBlockedUserAIImageHost("cdn.example.com") {
		t.Fatal("expected public hostname to be allowed")
	}
}

func containsAll(input string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(input, sub) {
			return false
		}
	}
	return true
}
