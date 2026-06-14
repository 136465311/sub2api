package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrAIConversationNotFound = infraerrors.NotFound("AI_CONVERSATION_NOT_FOUND", "chat conversation not found")
	ErrAIGroupNotAvailable    = infraerrors.Forbidden("AI_GROUP_NOT_AVAILABLE", "group is not available for this user")
	ErrAIModelRequired        = infraerrors.BadRequest("AI_MODEL_REQUIRED", "model is required")
)

type ChatConversation struct {
	ID        int64
	UserID    int64
	GroupID   *int64
	Title     string
	Model     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []ChatMessage
}

type ChatMessage struct {
	ID             int64
	ConversationID int64
	UserID         int64
	Role           string
	Content        string
	Model          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AIModelGroup struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Platform string   `json:"platform"`
	Models   []string `json:"models"`
}

type AIModelsResult struct {
	Groups         []AIModelGroup `json:"groups"`
	DefaultGroupID *int64         `json:"default_group_id,omitempty"`
	DefaultModel   string         `json:"default_model,omitempty"`
}

type ChatConversationCreateInput struct {
	UserID  int64
	GroupID *int64
	Title   string
	Model   string
}

type ChatMessageCreateInput struct {
	ConversationID int64
	UserID         int64
	Role           string
	Content        string
	Model          string
}

type UserAIRepository interface {
	ListChatConversations(ctx context.Context, userID int64, params pagination.PaginationParams, messagesLimit int) ([]ChatConversation, *pagination.PaginationResult, error)
	GetChatConversation(ctx context.Context, userID, conversationID int64) (*ChatConversation, error)
	CreateChatConversation(ctx context.Context, input ChatConversationCreateInput) (*ChatConversation, error)
	DeleteChatConversation(ctx context.Context, userID, conversationID int64) error
	CreateChatMessage(ctx context.Context, input ChatMessageCreateInput) (*ChatMessage, error)
	UpdateChatConversationAfterMessage(ctx context.Context, userID, conversationID int64, title, model string, groupID *int64) error
}

type UserAIService struct {
	repo           UserAIRepository
	apiKeyService  *APIKeyService
	gatewayService *GatewayService
}

func NewUserAIService(repo UserAIRepository, apiKeyService *APIKeyService, gatewayService *GatewayService) *UserAIService {
	return &UserAIService{repo: repo, apiKeyService: apiKeyService, gatewayService: gatewayService}
}

func (s *UserAIService) ListModels(ctx context.Context, userID int64) (*AIModelsResult, error) {
	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &AIModelsResult{Groups: make([]AIModelGroup, 0, len(groups))}
	for i := range groups {
		group := groups[i]
		groupID := group.ID
		models := []string(nil)
		if s.gatewayService != nil {
			models = s.gatewayService.GetAvailableModels(ctx, &groupID, group.Platform)
		}
		if len(models) == 0 && strings.TrimSpace(group.DefaultMappedModel) != "" {
			models = []string{strings.TrimSpace(group.DefaultMappedModel)}
		}
		result.Groups = append(result.Groups, AIModelGroup{
			ID:       group.ID,
			Name:     group.Name,
			Platform: group.Platform,
			Models:   models,
		})
		if result.DefaultGroupID == nil {
			result.DefaultGroupID = &groupID
			if len(models) > 0 {
				result.DefaultModel = models[0]
			}
		}
	}
	return result, nil
}

func (s *UserAIService) ResolveGroup(ctx context.Context, userID int64, requestedGroupID *int64) (*Group, error) {
	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, ErrAIGroupNotAvailable
	}
	if requestedGroupID != nil {
		for i := range groups {
			if groups[i].ID == *requestedGroupID {
				return &groups[i], nil
			}
		}
		return nil, ErrAIGroupNotAvailable
	}
	return &groups[0], nil
}

func (s *UserAIService) GetOrCreateInternalKey(ctx context.Context, userID int64, groupID *int64) (*APIKey, error) {
	return s.apiKeyService.GetOrCreateUserAIInternalKey(ctx, userID, groupID)
}

func (s *UserAIService) ListChatConversations(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ChatConversation, *pagination.PaginationResult, error) {
	return s.repo.ListChatConversations(ctx, userID, params, 100)
}

func (s *UserAIService) CreateChatConversation(ctx context.Context, input ChatConversationCreateInput) (*ChatConversation, error) {
	input.Title = normalizeConversationTitle(input.Title)
	input.Model = strings.TrimSpace(input.Model)
	return s.repo.CreateChatConversation(ctx, input)
}

func (s *UserAIService) DeleteChatConversation(ctx context.Context, userID, conversationID int64) error {
	return s.repo.DeleteChatConversation(ctx, userID, conversationID)
}

func (s *UserAIService) EnsureChatConversation(ctx context.Context, userID int64, conversationID int64, groupID *int64, model, titleSeed string) (*ChatConversation, error) {
	if conversationID > 0 {
		return s.repo.GetChatConversation(ctx, userID, conversationID)
	}
	return s.CreateChatConversation(ctx, ChatConversationCreateInput{
		UserID:  userID,
		GroupID: groupID,
		Title:   titleSeed,
		Model:   model,
	})
}

func (s *UserAIService) SaveChatTurn(ctx context.Context, userID int64, conversationID int64, groupID *int64, model, userContent, assistantContent string) error {
	if conversationID <= 0 {
		return fmt.Errorf("conversation id is required")
	}
	userContent = strings.TrimSpace(userContent)
	titleSeed, hasImage := chatMessageContentSummary(userContent)
	if titleSeed == "" && hasImage {
		titleSeed = "图片消息"
	}
	title := normalizeConversationTitle(titleSeed)
	if strings.TrimSpace(userContent) != "" {
		if _, err := s.repo.CreateChatMessage(ctx, ChatMessageCreateInput{
			ConversationID: conversationID,
			UserID:         userID,
			Role:           "user",
			Content:        userContent,
			Model:          strings.TrimSpace(model),
		}); err != nil {
			return err
		}
	}
	if strings.TrimSpace(assistantContent) != "" {
		if _, err := s.repo.CreateChatMessage(ctx, ChatMessageCreateInput{
			ConversationID: conversationID,
			UserID:         userID,
			Role:           "assistant",
			Content:        strings.TrimSpace(assistantContent),
			Model:          strings.TrimSpace(model),
		}); err != nil {
			return err
		}
	}
	return s.repo.UpdateChatConversationAfterMessage(ctx, userID, conversationID, title, strings.TrimSpace(model), groupID)
}

func chatMessageContentSummary(content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", false
	}
	if !strings.HasPrefix(content, "[") {
		return content, false
	}

	var parts []any
	if err := json.Unmarshal([]byte(content), &parts); err != nil {
		return content, false
	}

	var textParts []string
	hasImage := false
	for _, part := range parts {
		switch typed := part.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				textParts = append(textParts, text)
			}
		case map[string]any:
			partType := strings.TrimSpace(stringFromMapValue(typed["type"]))
			if (partType == "text" || partType == "input_text" || typed["text"] != nil) && strings.TrimSpace(stringFromMapValue(typed["text"])) != "" {
				textParts = append(textParts, strings.TrimSpace(stringFromMapValue(typed["text"])))
			}
			if partType == "image_url" || mapValueHasImageURL(typed) {
				hasImage = true
			}
		}
	}
	return strings.Join(textParts, "\n"), hasImage
}

func stringFromMapValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func mapValueHasImageURL(value map[string]any) bool {
	if strings.TrimSpace(stringFromMapValue(value["url"])) != "" || strings.TrimSpace(stringFromMapValue(value["image_url"])) != "" {
		return true
	}
	imageURL, ok := value["image_url"].(map[string]any)
	if !ok {
		return false
	}
	return strings.TrimSpace(stringFromMapValue(imageURL["url"])) != ""
}

func normalizeConversationTitle(title string) string {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	if title == "" {
		return "新会话"
	}
	const maxRunes = 60
	runes := []rune(title)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return title
}
