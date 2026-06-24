package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type userAIRepoStub struct {
	messages      []ChatMessageCreateInput
	updateTitle   string
	conversations map[int64]*ChatConversation
}

func (s *userAIRepoStub) ListChatConversations(context.Context, int64, pagination.PaginationParams, int) ([]ChatConversation, *pagination.PaginationResult, error) {
	panic("unexpected ListChatConversations call")
}

func (s *userAIRepoStub) GetChatConversation(_ context.Context, userID, conversationID int64) (*ChatConversation, error) {
	if s.conversations == nil {
		return nil, ErrAIConversationNotFound
	}
	conversation, ok := s.conversations[conversationID]
	if !ok || conversation == nil || conversation.UserID != userID {
		return nil, ErrAIConversationNotFound
	}
	copyConversation := *conversation
	copyConversation.Messages = append([]ChatMessage(nil), conversation.Messages...)
	return &copyConversation, nil
}

func (s *userAIRepoStub) CreateChatConversation(context.Context, ChatConversationCreateInput) (*ChatConversation, error) {
	panic("unexpected CreateChatConversation call")
}

func (s *userAIRepoStub) DeleteChatConversation(context.Context, int64, int64) error {
	panic("unexpected DeleteChatConversation call")
}

func (s *userAIRepoStub) CreateChatMessage(_ context.Context, input ChatMessageCreateInput) (*ChatMessage, error) {
	s.messages = append(s.messages, input)
	return &ChatMessage{
		ID:             int64(len(s.messages)),
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		Role:           input.Role,
		Content:        input.Content,
		Model:          input.Model,
	}, nil
}

func (s *userAIRepoStub) UpdateChatConversationAfterMessage(_ context.Context, _ int64, _ int64, title, _ string, _ *int64) error {
	s.updateTitle = title
	return nil
}

func (s *userAIRepoStub) CreateImageGenerationHistory(context.Context, ImageGenerationHistoryCreateInput) (*ImageGenerationHistory, error) {
	panic("unexpected CreateImageGenerationHistory call")
}

func (s *userAIRepoStub) ListImageGenerationHistory(context.Context, int64, pagination.PaginationParams) ([]ImageGenerationHistory, *pagination.PaginationResult, error) {
	panic("unexpected ListImageGenerationHistory call")
}

func TestUserAIServiceSaveChatTurnPreservesMultimodalUserContent(t *testing.T) {
	repo := &userAIRepoStub{}
	svc := NewUserAIService(repo, nil, nil)
	content := `[{"type":"text","text":"Describe this"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/image.png"}}]`

	if err := svc.SaveChatTurn(context.Background(), 7, 11, nil, "gpt-4o", content, "It is a chart."); err != nil {
		t.Fatalf("SaveChatTurn returned error: %v", err)
	}

	if len(repo.messages) != 2 {
		t.Fatalf("expected 2 saved messages, got %d", len(repo.messages))
	}
	if repo.messages[0].Role != "user" || repo.messages[0].Content != content {
		t.Fatalf("user message not preserved: %#v", repo.messages[0])
	}
	if repo.updateTitle != "Describe this" {
		t.Fatalf("title should use text part only, got %q", repo.updateTitle)
	}
}

func TestChatMessageContentSummaryUsesImageFallback(t *testing.T) {
	title, hasImage := chatMessageContentSummary(`[{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/image.png"}}]`)
	if title != "" || !hasImage {
		t.Fatalf("expected empty title and image flag, got title=%q hasImage=%v", title, hasImage)
	}
}

func TestResolveAIGroupFromAvailableGroupsMatchesByIDWithoutImageFlag(t *testing.T) {
	groupID := int64(12)
	groups := []Group{{
		ID:                   groupID,
		Name:                 "chatgpt-plus",
		Platform:             PlatformOpenAI,
		AllowImageGeneration: false,
	}}

	got, err := resolveAIGroupFromAvailableGroups(groups, AIGroupRequest{GroupID: &groupID})
	if err != nil {
		t.Fatalf("resolveAIGroupFromAvailableGroups returned error: %v", err)
	}
	if got == nil || got.ID != groupID {
		t.Fatalf("expected group %d, got %#v", groupID, got)
	}
}

func TestResolveAIGroupFromAvailableGroupsMatchesByName(t *testing.T) {
	groups := []Group{
		{ID: 7, Name: "default", Platform: PlatformOpenAI},
		{ID: 12, Name: "chatgpt-plus", Platform: PlatformOpenAI},
	}

	got, err := resolveAIGroupFromAvailableGroups(groups, AIGroupRequest{GroupName: " CHATGPT-PLUS "})
	if err != nil {
		t.Fatalf("resolveAIGroupFromAvailableGroups returned error: %v", err)
	}
	if got == nil || got.ID != 12 {
		t.Fatalf("expected chatgpt-plus group, got %#v", got)
	}
}

func TestResolveAIGroupFromAvailableGroupsRejectsUnavailableGroup(t *testing.T) {
	groupID := int64(99)
	groups := []Group{{ID: 12, Name: "chatgpt-plus", Platform: PlatformOpenAI}}

	_, err := resolveAIGroupFromAvailableGroups(groups, AIGroupRequest{GroupID: &groupID})
	if !errors.Is(err, ErrAIGroupNotAvailable) {
		t.Fatalf("expected ErrAIGroupNotAvailable, got %v", err)
	}
}

func TestIsOpenAIImageModelName(t *testing.T) {
	tests := map[string]bool{
		"gpt-image":     true,
		"gpt-image-2":   true,
		"grok-image":    true,
		"grok-image-v2": true,
		"gpt-4o":        false,
	}

	for model, want := range tests {
		if got := isOpenAIImageModelName(model); got != want {
			t.Fatalf("isOpenAIImageModelName(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestResolveImageConversationContextPrefersAssistantImage(t *testing.T) {
	repo := &userAIRepoStub{
		conversations: map[int64]*ChatConversation{
			11: {
				ID:     11,
				UserID: 7,
				Messages: []ChatMessage{
					{Role: "user", Content: `[{"type":"text","text":"把两个人结合到一块"}]`},
					{Role: "assistant", Content: `[{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/generated/result.png"}}]`},
				},
			},
		},
	}
	svc := NewUserAIService(repo, nil, nil)
	ctx, err := svc.ResolveImageConversationContext(context.Background(), 7, 11, "让两个人脸变成证件照一样")
	if err != nil {
		t.Fatalf("ResolveImageConversationContext returned error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected context")
	}
	if len(ctx.ReferenceImageURLs) != 1 || ctx.ReferenceImageURLs[0] != "/uploads/user_ai/7/generated/result.png" {
		t.Fatalf("unexpected reference images: %#v", ctx.ReferenceImageURLs)
	}
	if !containsAll(ctx.Prompt, "把两个人结合到一块", "让两个人脸变成证件照一样") {
		t.Fatalf("expected merged prompt, got %q", ctx.Prompt)
	}
	if !containsAll(ctx.Prompt, "基于上一轮图片继续修改。", "上一轮要求：", "本轮要求：") {
		t.Fatalf("expected continuation prompt, got %q", ctx.Prompt)
	}
}

func TestResolveImageConversationContextFallsBackToUserUpload(t *testing.T) {
	repo := &userAIRepoStub{
		conversations: map[int64]*ChatConversation{
			12: {
				ID:     12,
				UserID: 7,
				Messages: []ChatMessage{
					{Role: "user", Content: `[{"type":"text","text":"把两个人结合到一块"},{"type":"image_url","image_url":{"url":"/uploads/user_ai/7/source.png"}}]`},
				},
			},
		},
	}
	svc := NewUserAIService(repo, nil, nil)
	ctx, err := svc.ResolveImageConversationContext(context.Background(), 7, 12, "改成证件照风格")
	if err != nil {
		t.Fatalf("ResolveImageConversationContext returned error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected context")
	}
	if len(ctx.ReferenceImageURLs) != 1 || ctx.ReferenceImageURLs[0] != "/uploads/user_ai/7/source.png" {
		t.Fatalf("unexpected reference images: %#v", ctx.ReferenceImageURLs)
	}
	if !containsAll(ctx.Prompt, "把两个人结合到一块", "改成证件照风格") {
		t.Fatalf("expected merged prompt, got %q", ctx.Prompt)
	}
	if !containsAll(ctx.Prompt, "基于上一轮图片继续修改。", "上一轮要求：", "本轮要求：") {
		t.Fatalf("expected continuation prompt, got %q", ctx.Prompt)
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
