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
	ErrAIImageRequired        = infraerrors.BadRequest("AI_IMAGE_REQUIRED", "prompt is required")
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

type ImageGenerationHistory struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	GroupID   *int64    `json:"group_id,omitempty"`
	Prompt    string    `json:"prompt"`
	Model     string    `json:"model"`
	Size      string    `json:"size"`
	N         int       `json:"n"`
	Images    []string  `json:"images"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageGenerationHistoryCreateInput struct {
	UserID  int64
	GroupID *int64
	Prompt  string
	Model   string
	Size    string
	N       int
	Images  []string
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
	CreateImageGenerationHistory(ctx context.Context, input ImageGenerationHistoryCreateInput) (*ImageGenerationHistory, error)
	ListImageGenerationHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageGenerationHistory, *pagination.PaginationResult, error)
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

func (s *UserAIService) ListImageModels(ctx context.Context, userID int64) (*AIModelsResult, error) {
	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &AIModelsResult{Groups: make([]AIModelGroup, 0, len(groups))}
	for i := range groups {
		group := groups[i]
		if strings.TrimSpace(group.Platform) != PlatformOpenAI {
			continue
		}
		if !GroupAllowsImageGeneration(&group) {
			continue
		}

		groupID := group.ID
		models := []string(nil)
		if s.gatewayService != nil {
			models = filterImageModels(s.gatewayService.GetAvailableModels(ctx, &groupID, group.Platform))
		}
		if len(models) == 0 && isOpenAIImageModelName(group.DefaultMappedModel) {
			models = []string{strings.TrimSpace(group.DefaultMappedModel)}
		}
		if len(models) == 0 {
			models = []string{"gpt-image-2"}
		}

		result.Groups = append(result.Groups, AIModelGroup{
			ID:       group.ID,
			Name:     group.Name,
			Platform: group.Platform,
			Models:   models,
		})
		if result.DefaultGroupID == nil {
			result.DefaultGroupID = &groupID
			result.DefaultModel = models[0]
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

func (s *UserAIService) ResolveImageGroup(ctx context.Context, userID int64, requestedGroupID *int64) (*Group, error) {
	groups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		group := groups[i]
		if requestedGroupID != nil && group.ID != *requestedGroupID {
			continue
		}
		if strings.TrimSpace(group.Platform) == PlatformOpenAI && GroupAllowsImageGeneration(&group) {
			return &group, nil
		}
	}
	return nil, ErrAIGroupNotAvailable
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

func (s *UserAIService) SaveImageGeneration(ctx context.Context, input ImageGenerationHistoryCreateInput) (*ImageGenerationHistory, error) {
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Model = strings.TrimSpace(input.Model)
	input.Size = strings.TrimSpace(input.Size)
	input.Images = compactStrings(input.Images)
	if input.Prompt == "" {
		return nil, ErrAIImageRequired
	}
	if input.N <= 0 {
		input.N = len(input.Images)
	}
	if input.N <= 0 {
		input.N = 1
	}
	return s.repo.CreateImageGenerationHistory(ctx, input)
}

func (s *UserAIService) ListImageGenerationHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageGenerationHistory, *pagination.PaginationResult, error) {
	return s.repo.ListImageGenerationHistory(ctx, userID, params)
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

func filterImageModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		trimmed := strings.TrimSpace(model)
		if !isOpenAIImageModelName(trimmed) {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func isOpenAIImageModelName(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return normalized == "gpt-image" || strings.HasPrefix(normalized, "gpt-image-") || strings.HasPrefix(normalized, "grok-image")
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
