package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestNormalizeUserAIImageSize(t *testing.T) {
	tests := map[string]string{
		"":          "auto",
		"1:1":       "1024x1024",
		"16:9":      "2048x1152",
		"9:16":      "1152x2048",
		"1024x1024": "1024x1024",
		"auto":      "auto",
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

func TestValidateUserAIEditImageURLs(t *testing.T) {
	got, err := validateUserAIEditImageURLs(7, []string{
		"/uploads/user_ai/7/a.png",
		"/uploads/user_ai/7/a.png",
		"/uploads/user_ai/7/generated/b.webp",
	})
	if err != nil {
		t.Fatalf("expected uploaded image URLs to be valid: %v", err)
	}
	if len(got) != 2 || got[0] != "/uploads/user_ai/7/a.png" || got[1] != "/uploads/user_ai/7/generated/b.webp" {
		t.Fatalf("unexpected normalized image urls: %#v", got)
	}

	tests := []struct {
		name string
		url  string
	}{
		{name: "data url", url: "data:image/png;base64,abc"},
		{name: "external url", url: "https://cdn.example/a.png"},
		{name: "protocol relative url", url: "//cdn.example/a.png"},
		{name: "cross user", url: "/uploads/user_ai/8/a.png"},
		{name: "bad path", url: "/uploads/other/7/a.png"},
		{name: "bad extension", url: "/uploads/user_ai/7/a.svg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := validateUserAIEditImageURLs(7, []string{tt.url}); err == nil {
				t.Fatalf("expected %q to be rejected", tt.url)
			}
		})
	}
}

func TestUserAIImageEditUserContent(t *testing.T) {
	got := userAIImageEditUserContent("replace the background", []string{"/uploads/user_ai/7/a.png"})
	var parts []map[string]any
	if err := json.Unmarshal([]byte(got), &parts); err != nil {
		t.Fatalf("content should be JSON parts: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected text and image parts, got %#v", parts)
	}
	if parts[0]["type"] != "text" || parts[0]["text"] != "replace the background" {
		t.Fatalf("unexpected text part: %#v", parts[0])
	}
	imageURL := parts[1]["image_url"].(map[string]any)["url"]
	if imageURL != "/uploads/user_ai/7/a.png" {
		t.Fatalf("unexpected image url: %v", imageURL)
	}
}

func TestPrepareImageGenerationsProxyUsesConversationHistoryReferences(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(12)
	conversationID := int64(42)
	apiKeyRepo := &userAIImageAPIKeyRepoStub{
		keys: []service.APIKey{{
			ID:      99,
			UserID:  7,
			Key:     "sk-user-ai-internal",
			Source:  service.APIKeySourceUserAI,
			GroupID: &groupID,
			Status:  service.StatusAPIKeyActive,
		}},
	}
	apiKeyService := service.NewAPIKeyService(
		apiKeyRepo,
		&userAIImageUserRepoStub{},
		&userAIImageGroupRepoStub{
			groups: []service.Group{{
				ID:       groupID,
				Name:     "image",
				Platform: service.PlatformOpenAI,
				Status:   service.StatusActive,
			}},
		},
		&userAIImageSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
	uploadRoot := t.TempDir()
	userDir := filepath.Join(uploadRoot, "7", "generated")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("create upload dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "result.png"), tinyPNG(), 0644); err != nil {
		t.Fatalf("write generated source image: %v", err)
	}
	h := &UserAIHandler{
		userAIService: service.NewUserAIService(&userAIImageConversationRepoStub{
			conversations: map[int64]*service.ChatConversation{
				conversationID: {
					ID:     conversationID,
					UserID: 7,
					Messages: []service.ChatMessage{
						{Role: "user", Content: `[{"type":"text","text":"keep the two people together"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/source.png"}}]`},
						{Role: "assistant", Content: `[{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/generated/result.png"}}]`},
					},
				},
			},
		}, apiKeyService, nil),
		uploadRoot:        uploadRoot,
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}

	body := `{"prompt":"make it id photo style","model":"gpt-image-2","size":"1:1","n":1,"group_id":12,"conversation_id":42}`
	rec := httptest.NewRecorder()
	nextCalled := false
	router := gin.New()
	router.POST("/api/v1/user/images/generations", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	}, h.PrepareImageGenerationsProxy, func(c *gin.Context) {
		nextCalled = true
		if got := c.Request.URL.Path; got != "/v1/images/edits" {
			t.Fatalf("path = %q, want /v1/images/edits", got)
		}
		mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content-type = %q", mediaType)
		}
		rewritten, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read rewritten body: %v", err)
		}
		form, err := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"]).ReadForm(userAIUploadMaxFileSize + (1 << 20))
		if err != nil {
			t.Fatalf("read multipart form: %v", err)
		}
		if got := form.Value["prompt"]; len(got) != 1 || !containsAll(got[0], "make it id photo style") {
			t.Fatalf("unexpected merged prompt: %#v", got)
		}
		files := form.File["image"]
		if len(files) != 1 {
			t.Fatalf("expected one reference image file, got %#v", form.File)
		}
		if files[0].Filename != "result.png" {
			t.Fatalf("expected assistant image reference, got %q", files[0].Filename)
		}
		if got := c.GetHeader("Authorization"); got != "Bearer sk-user-ai-internal" {
			t.Fatalf("authorization header = %q", got)
		}
		contextRefs := stringSliceFromContext(c, userAIImageRefsContextKey)
		if len(contextRefs) != 1 || contextRefs[0] != "/uploads/user_ai/7/generated/result.png" {
			t.Fatalf("unexpected context refs: %#v", contextRefs)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "https://chat.example/api/v1/user/images/generations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("PrepareImageGenerationsProxy returned status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestPrepareImageGenerationsProxyUsesImageDefaultOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(12)
	apiKeyRepo := &userAIImageAPIKeyRepoStub{
		keys: []service.APIKey{{
			ID:      99,
			UserID:  7,
			Key:     "sk-user-ai-internal",
			Source:  service.APIKeySourceUserAI,
			GroupID: &groupID,
			Status:  service.StatusAPIKeyActive,
		}},
	}
	apiKeyService := service.NewAPIKeyService(
		apiKeyRepo,
		&userAIImageUserRepoStub{},
		&userAIImageGroupRepoStub{
			groups: []service.Group{{
				ID:       groupID,
				Name:     "image",
				Platform: service.PlatformOpenAI,
				Status:   service.StatusActive,
			}},
		},
		&userAIImageSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
	h := &UserAIHandler{
		userAIService:     service.NewUserAIService(&userAIImageRepoStub{}, apiKeyService, nil),
		uploadRoot:        t.TempDir(),
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}

	body := `{"prompt":"draw a blog banner","model":"gpt-image-2","n":1,"group_id":12}`
	rec := httptest.NewRecorder()
	nextCalled := false
	router := gin.New()
	router.POST("/api/v1/user/images/generations", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	}, h.PrepareImageGenerationsProxy, func(c *gin.Context) {
		nextCalled = true
		if got := c.Request.URL.Path; got != "/v1/images/generations" {
			t.Fatalf("path = %q, want /v1/images/generations", got)
		}
		if got := c.GetHeader("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q", got)
		}
		if got := c.GetHeader("Authorization"); got != "Bearer sk-user-ai-internal" {
			t.Fatalf("authorization header = %q", got)
		}
		rewritten, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read rewritten body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(rewritten, &payload); err != nil {
			t.Fatalf("rewritten body should be JSON: %v", err)
		}
		if got := payload["size"]; got != "auto" {
			t.Fatalf("size = %#v, want auto; body=%s", got, string(rewritten))
		}
		if got := payload["quality"]; got != "auto" {
			t.Fatalf("quality = %#v, want auto; body=%s", got, string(rewritten))
		}
		if got := payload["moderation"]; got != "auto" {
			t.Fatalf("moderation = %#v, want auto; body=%s", got, string(rewritten))
		}
		if got := payload["output_format"]; got != "png" {
			t.Fatalf("output_format = %#v, want png; body=%s", got, string(rewritten))
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "https://chat.example/api/v1/user/images/generations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("PrepareImageGenerationsProxy returned status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestPrepareImageEditsProxyBuildsMultipartFromUploadedImageURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(12)
	apiKeyRepo := &userAIImageAPIKeyRepoStub{
		keys: []service.APIKey{{
			ID:      99,
			UserID:  7,
			Key:     "sk-user-ai-internal",
			Source:  service.APIKeySourceUserAI,
			GroupID: &groupID,
			Status:  service.StatusAPIKeyActive,
		}},
	}
	apiKeyService := service.NewAPIKeyService(
		apiKeyRepo,
		&userAIImageUserRepoStub{},
		&userAIImageGroupRepoStub{
			groups: []service.Group{{
				ID:       groupID,
				Name:     "image",
				Platform: service.PlatformOpenAI,
				Status:   service.StatusActive,
			}},
		},
		&userAIImageSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
	uploadRoot := t.TempDir()
	userDir := filepath.Join(uploadRoot, "7")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("create upload dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "source.png"), tinyPNG(), 0644); err != nil {
		t.Fatalf("write uploaded source image: %v", err)
	}
	h := &UserAIHandler{
		userAIService:     service.NewUserAIService(&userAIImageRepoStub{}, apiKeyService, nil),
		uploadRoot:        uploadRoot,
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}

	body := `{"prompt":"replace background","model":"gpt-image-2","size":"1:1","n":2,"group_id":12,"conversation_id":42,"image_urls":["/uploads/user_ai/7/source.png"]}`
	rec := httptest.NewRecorder()
	nextCalled := false
	router := gin.New()
	router.POST("/api/v1/user/images/edits", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	}, h.PrepareImageEditsProxy, func(c *gin.Context) {
		nextCalled = true
		if got := c.Request.URL.Path; got != "/v1/images/edits" {
			t.Fatalf("path = %q, want /v1/images/edits", got)
		}
		if got := c.GetHeader("Authorization"); got != "Bearer sk-user-ai-internal" {
			t.Fatalf("authorization header = %q", got)
		}

		rewritten, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read rewritten body: %v", err)
		}
		mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil {
			t.Fatalf("parse multipart content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content-type = %q, want multipart/form-data", mediaType)
		}
		form, err := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"]).ReadForm(userAIUploadMaxFileSize + (1 << 20))
		if err != nil {
			t.Fatalf("read multipart form: %v", err)
		}
		if got := form.Value["prompt"]; len(got) != 1 || got[0] != "replace background" {
			t.Fatalf("prompt field = %#v", got)
		}
		if got := form.Value["model"]; len(got) != 1 || got[0] != "gpt-image-2" {
			t.Fatalf("model field = %#v", got)
		}
		if got := form.Value["size"]; len(got) != 1 || got[0] != "1024x1024" {
			t.Fatalf("size field = %#v", got)
		}
		if got := form.Value["quality"]; len(got) != 1 || got[0] != "auto" {
			t.Fatalf("quality field = %#v", got)
		}
		if got := form.Value["output_format"]; len(got) != 1 || got[0] != "png" {
			t.Fatalf("output_format field = %#v", got)
		}
		if got := form.Value["moderation"]; len(got) != 1 || got[0] != "auto" {
			t.Fatalf("moderation field = %#v", got)
		}
		if got := form.Value["n"]; len(got) != 1 || got[0] != "2" {
			t.Fatalf("n field = %#v", got)
		}
		if got := form.Value["response_format"]; len(got) != 1 || got[0] != "url" {
			t.Fatalf("response_format field = %#v", got)
		}
		files := form.File["image"]
		if len(files) != 1 {
			t.Fatalf("expected one image file part, got %#v", form.File)
		}
		if files[0].Filename != "source.png" {
			t.Fatalf("image filename = %q", files[0].Filename)
		}
		if got := files[0].Header.Get("Content-Type"); got != "image/png" {
			t.Fatalf("image content-type = %q", got)
		}
		file, err := files[0].Open()
		if err != nil {
			t.Fatalf("open image file part: %v", err)
		}
		defer func() { _ = file.Close() }()
		imageData, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read image file part: %v", err)
		}
		if !bytes.Equal(imageData, tinyPNG()) {
			t.Fatalf("multipart image part did not contain uploaded image bytes")
		}

		userContent := userContentStringFromContext(c, userAIImageUserContextKey)
		if !containsAll(userContent, "replace background", "/uploads/user_ai/7/source.png") {
			t.Fatalf("edit user content should preserve prompt and relative source URL: %s", userContent)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "https://chat.example/api/v1/user/images/edits", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("PrepareImageEditsProxy returned status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
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

type userAIImageRepoStub struct {
	service.UserAIRepository
}

type userAIImageConversationRepoStub struct {
	service.UserAIRepository
	conversations map[int64]*service.ChatConversation
}

func (s *userAIImageConversationRepoStub) GetChatConversation(_ context.Context, userID, conversationID int64) (*service.ChatConversation, error) {
	conversation, ok := s.conversations[conversationID]
	if !ok || conversation == nil || conversation.UserID != userID {
		return nil, service.ErrAIConversationNotFound
	}
	copyConversation := *conversation
	copyConversation.Messages = append([]service.ChatMessage(nil), conversation.Messages...)
	return &copyConversation, nil
}

type userAIImageAPIKeyRepoStub struct {
	service.APIKeyRepository
	keys []service.APIKey
}

func (s *userAIImageAPIKeyRepoStub) GetBySourceForUserGroup(_ context.Context, userID int64, groupID *int64, source string) (*service.APIKey, error) {
	for i := range s.keys {
		key := &s.keys[i]
		if key.UserID != userID || key.Source != source {
			continue
		}
		if !optionalInt64Equal(key.GroupID, groupID) {
			continue
		}
		return key, nil
	}
	return nil, service.ErrAPIKeyNotFound
}

func (s *userAIImageAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, _ pagination.PaginationParams, _ service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	var out []service.APIKey
	for _, key := range s.keys {
		if key.UserID == userID {
			out = append(out, key)
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out))}, nil
}

type userAIImageUserRepoStub struct {
	service.UserRepository
}

func (s *userAIImageUserRepoStub) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 7, Role: service.RoleUser}, nil
}

type userAIImageGroupRepoStub struct {
	service.GroupRepository
	groups []service.Group
}

func (s *userAIImageGroupRepoStub) GetByID(_ context.Context, id int64) (*service.Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			return &s.groups[i], nil
		}
	}
	return nil, service.ErrGroupNotFound
}

func (s *userAIImageGroupRepoStub) ListActive(context.Context) ([]service.Group, error) {
	return append([]service.Group(nil), s.groups...), nil
}

type userAIImageSubscriptionRepoStub struct {
	service.UserSubscriptionRepository
}

func (s *userAIImageSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return nil, nil
}

func optionalInt64Equal(a, b *int64) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
