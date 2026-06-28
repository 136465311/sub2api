package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	userAIConversationIDContextKey = "_user_ai_conversation_id"
	userAIUserMessageContextKey    = "_user_ai_user_message"
	userAIUserContentContextKey    = "_user_ai_user_content"
	userAIModelContextKey          = "_user_ai_model"
	userAIGroupIDContextKey        = "_user_ai_group_id"
)

type UserAIHandler struct {
	userAIService     *service.UserAIService
	gateway           *GatewayHandler
	openAIGateway     *OpenAIGatewayHandler
	uploadRoot        string
	uploadPublicRoot  string
	uploadMaxFileSize int64
}

func NewUserAIHandler(userAIService *service.UserAIService, gateway *GatewayHandler, openAIGateway *OpenAIGatewayHandler) *UserAIHandler {
	return &UserAIHandler{
		userAIService:     userAIService,
		gateway:           gateway,
		openAIGateway:     openAIGateway,
		uploadRoot:        userAIUploadRoot,
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}
}

func (h *UserAIHandler) Models(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.userAIService.ListModels(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAIHandler) ListChatConversations(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	conversations, pageResult, err := h.userAIService.ListChatConversations(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, conversations, pageResult.Total, page, pageSize)
}

func (h *UserAIHandler) CreateChatConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req struct {
		Title   string `json:"title"`
		Model   string `json:"model"`
		GroupID *int64 `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.GroupID != nil {
		if _, err := h.userAIService.ResolveGroup(c.Request.Context(), subject.UserID, req.GroupID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	conversation, err := h.userAIService.CreateChatConversation(c.Request.Context(), service.ChatConversationCreateInput{
		UserID:  subject.UserID,
		GroupID: req.GroupID,
		Title:   req.Title,
		Model:   req.Model,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, conversation)
}

func (h *UserAIHandler) UpdateChatConversationTitle(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		response.BadRequest(c, "title is required")
		return
	}

	conversation, err := h.userAIService.UpdateChatConversationTitle(c.Request.Context(), service.ChatConversationTitleUpdateInput{
		UserID:         subject.UserID,
		ConversationID: id,
		Title:          req.Title,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, conversation)
}

func (h *UserAIHandler) DeleteChatConversation(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid conversation ID")
		return
	}
	if err := h.userAIService.DeleteChatConversation(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *UserAIHandler) PrepareChatCompletionsProxy(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		c.Abort()
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Failed to read request body")
		c.Abort()
		return
	}
	if len(bytes.TrimSpace(body)) == 0 || !gjson.ValidBytes(body) {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}
	if userAIRequestHasDataImageURL(body) {
		response.BadRequest(c, "Images must be uploaded first and referenced by image_url")
		c.Abort()
		return
	}

	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		response.ErrorFrom(c, service.ErrAIModelRequired)
		c.Abort()
		return
	}
	groupRequest := parseUserAIGroupRequest(payload["group_id"], payload["group_name"], payload["group"])
	group, err := h.userAIService.ResolveRequestedGroup(c.Request.Context(), subject.UserID, groupRequest)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}
	resolvedGroupID := group.ID

	userContent := extractLastUserMessage(body)
	userContentForStorage := extractLastUserMessageRawContent(body)
	conversationID := parseOptionalInt64Value(payload["conversation_id"])
	ephemeral := parseOptionalBoolValue(payload["user_ai_ephemeral"]) ||
		parseOptionalBoolValue(gjson.GetBytes(body, "metadata.user_ai_ephemeral").Value())
	var resolvedConversationID int64
	if !ephemeral {
		conversation, err := h.userAIService.EnsureChatConversation(c.Request.Context(), subject.UserID, conversationID, &resolvedGroupID, model, userContent)
		if err != nil {
			response.ErrorFrom(c, err)
			c.Abort()
			return
		}
		resolvedConversationID = conversation.ID
	}

	internalKey, err := h.userAIService.GetOrCreateInternalKey(c.Request.Context(), subject.UserID, &resolvedGroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}

	delete(payload, "conversation_id")
	delete(payload, "group_id")
	delete(payload, "group_name")
	delete(payload, "group")
	deleteUserAIEphemeralFields(payload)
	rewriteUserAIRelativeImageURLs(payload, userAIRequestBaseURL(c))
	cleanBody, err := json.Marshal(payload)
	if err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}

	if !ephemeral {
		c.Set(userAIConversationIDContextKey, resolvedConversationID)
		c.Set(userAIUserMessageContextKey, userContent)
		c.Set(userAIUserContentContextKey, userContentForStorage)
		c.Set(userAIModelContextKey, model)
		c.Set(userAIGroupIDContextKey, resolvedGroupID)
	}
	c.Request.Header.Set("Authorization", "Bearer "+internalKey.Key)
	c.Request.Header.Del("x-api-key")
	c.Request.Header.Del("x-goog-api-key")
	c.Request.Body = io.NopCloser(bytes.NewReader(cleanBody))
	c.Request.ContentLength = int64(len(cleanBody))
	c.Next()
}

func (h *UserAIHandler) ChatCompletions(c *gin.Context) {
	capture := &captureResponseWriter{ResponseWriter: c.Writer}
	c.Writer = capture

	if getUserAIGroupPlatform(c) == service.PlatformOpenAI {
		h.openAIGateway.ChatCompletions(c)
	} else {
		h.gateway.ChatCompletions(c)
	}

	status := capture.Status()
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return
	}

	conversationID, _ := getContextInt64Value(c, userAIConversationIDContextKey)
	groupID, _ := getContextInt64Value(c, userAIGroupIDContextKey)
	model, _ := c.Get(userAIModelContextKey)
	userContent, _ := c.Get(userAIUserMessageContextKey)
	userContentForStorage, _ := c.Get(userAIUserContentContextKey)
	assistantContent := extractAssistantContent(capture.body.String())
	if conversationID <= 0 || (strings.TrimSpace(userContentString(userContent)) == "" && strings.TrimSpace(userContentString(userContentForStorage)) == "") {
		return
	}
	_ = h.userAIService.SaveChatTurn(c.Request.Context(), userIDFromContext(c), conversationID, &groupID, stringFromAny(model), userContentString(userContentForStorage), assistantContent)
}

type captureResponseWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *captureResponseWriter) WriteString(data string) (int, error) {
	w.body.WriteString(data)
	return w.ResponseWriter.WriteString(data)
}

func getUserAIGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}

func userIDFromContext(c *gin.Context) int64 {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

func parseOptionalInt64(v any) *int64 {
	value := parseOptionalInt64Value(v)
	if value <= 0 {
		return nil
	}
	return &value
}

func parseOptionalInt64Value(v any) int64 {
	switch typed := v.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return n
	default:
		return 0
	}
}

func parseOptionalBoolValue(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func deleteUserAIEphemeralFields(payload map[string]any) {
	delete(payload, "user_ai_ephemeral")
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		return
	}
	delete(metadata, "user_ai_ephemeral")
	if len(metadata) == 0 {
		delete(payload, "metadata")
	}
}

func getContextInt64Value(c *gin.Context, key string) (int64, bool) {
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	return parseOptionalInt64Value(v), true
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func userContentString(v any) string {
	return strings.TrimSpace(stringFromAny(v))
}

func extractLastUserMessage(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return ""
	}
	values := messages.Array()
	for i := len(values) - 1; i >= 0; i-- {
		if values[i].Get("role").String() == "user" {
			return extractMessageContent(values[i].Get("content"))
		}
	}
	return ""
}

func extractLastUserMessageRawContent(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return ""
	}
	values := messages.Array()
	for i := len(values) - 1; i >= 0; i-- {
		if values[i].Get("role").String() != "user" {
			continue
		}
		content := values[i].Get("content")
		if content.Type == gjson.String {
			return strings.TrimSpace(content.String())
		}
		return strings.TrimSpace(content.Raw)
	}
	return ""
}

func extractMessageContent(content gjson.Result) string {
	if content.Type == gjson.String {
		return strings.TrimSpace(content.String())
	}
	if content.IsArray() {
		var parts []string
		for _, item := range content.Array() {
			switch item.Get("type").String() {
			case "text", "input_text":
				if text := strings.TrimSpace(item.Get("text").String()); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return strings.TrimSpace(content.Raw)
}

func userAIRequestHasDataImageURL(body []byte) bool {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return false
	}
	for _, message := range messages.Array() {
		content := message.Get("content")
		if !content.IsArray() {
			continue
		}
		for _, item := range content.Array() {
			if userAIContentPartHasDataImageURL(item) {
				return true
			}
		}
	}
	return false
}

func userAIContentPartHasDataImageURL(item gjson.Result) bool {
	if isUserAIDataImageURL(item.Get("image_url.url").String()) {
		return true
	}
	if isUserAIDataImageURL(item.Get("image_url").String()) {
		return true
	}
	return isUserAIDataImageURL(item.Get("url").String())
}

func isUserAIDataImageURL(url string) bool {
	normalized := strings.ToLower(strings.TrimSpace(url))
	if !strings.HasPrefix(normalized, "data:image/") {
		return false
	}
	return strings.Contains(normalized, ";base64,")
}

func rewriteUserAIRelativeImageURLs(payload map[string]any, baseURL string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return
	}
	messages, ok := payload["messages"].([]any)
	if !ok {
		return
	}
	for _, message := range messages {
		record, ok := message.(map[string]any)
		if !ok {
			continue
		}
		content, ok := record["content"].([]any)
		if !ok {
			continue
		}
		for _, part := range content {
			partRecord, ok := part.(map[string]any)
			if !ok {
				continue
			}
			rewriteUserAIImageURLField(partRecord, "url", baseURL)
			switch imageURL := partRecord["image_url"].(type) {
			case string:
				if rewritten := userAIAbsoluteImageURL(imageURL, baseURL); rewritten != "" {
					partRecord["image_url"] = rewritten
				}
			case map[string]any:
				rewriteUserAIImageURLField(imageURL, "url", baseURL)
			}
		}
	}
}

func rewriteUserAIImageURLField(record map[string]any, key, baseURL string) {
	value, ok := record[key].(string)
	if !ok {
		return
	}
	if rewritten := userAIAbsoluteImageURL(value, baseURL); rewritten != "" {
		record[key] = rewritten
	}
}

func userAIAbsoluteImageURL(url, baseURL string) string {
	trimmed := strings.TrimSpace(url)
	if !strings.HasPrefix(trimmed, userAIUploadPublicRoot+"/") {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + trimmed
}

func userAIRequestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		return ""
	}
	proto := firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return strings.ToLower(proto) + "://" + host
}

func firstForwardedHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func extractAssistantContent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "data:") || strings.Contains(raw, "\ndata:") {
		return extractAssistantContentFromSSE(raw)
	}
	if !gjson.Valid(raw) {
		return ""
	}
	if content := gjson.Get(raw, "choices.0.message.content").String(); content != "" {
		return content
	}
	if content := gjson.Get(raw, "content.0.text").String(); content != "" {
		return content
	}
	return ""
}

func extractAssistantContentFromSSE(raw string) string {
	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" || !gjson.Valid(data) {
			continue
		}
		if content := gjson.Get(data, "choices.0.delta.content").String(); content != "" {
			builder.WriteString(content)
			continue
		}
		if content := gjson.Get(data, "choices.0.message.content").String(); content != "" {
			builder.WriteString(content)
			continue
		}
		if content := gjson.Get(data, "delta.text").String(); content != "" {
			builder.WriteString(content)
			continue
		}
	}
	return builder.String()
}
