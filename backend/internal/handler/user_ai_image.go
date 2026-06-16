package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	userAIImagePromptContextKey = "_user_ai_image_prompt"
	userAIImageModelContextKey  = "_user_ai_image_model"
	userAIImageSizeContextKey   = "_user_ai_image_size"
	userAIImageCountContextKey  = "_user_ai_image_count"
	userAIImageUserContextKey   = "_user_ai_image_user_content"
)

type userAIImageGenerationRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model"`
	Size           string `json:"size"`
	N              int    `json:"n"`
	GroupID        any    `json:"group_id"`
	GroupName      any    `json:"group_name"`
	Group          any    `json:"group"`
	ConversationID any    `json:"conversation_id"`
}

type userAIImageEditRequest struct {
	Prompt         string   `json:"prompt"`
	Model          string   `json:"model"`
	Size           string   `json:"size"`
	N              int      `json:"n"`
	GroupID        any      `json:"group_id"`
	GroupName      any      `json:"group_name"`
	Group          any      `json:"group"`
	ConversationID any      `json:"conversation_id"`
	ImageURLs      []string `json:"image_urls"`
}

var userAIEditImageURLPattern = regexp.MustCompile(`^/uploads/user_ai/(\d+)/(?:generated/)?[A-Za-z0-9._-]+\.(?i:jpg|jpeg|png|webp|gif)$`)

func (h *UserAIHandler) ImageModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.userAIService.ListImageModels(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *UserAIHandler) PrepareImageGenerationsProxy(c *gin.Context) {
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

	var req userAIImageGenerationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Model = strings.TrimSpace(req.Model)
	req.Size = normalizeUserAIImageSize(req.Size)
	if req.Prompt == "" {
		response.ErrorFrom(c, service.ErrAIImageRequired)
		c.Abort()
		return
	}
	if req.Model == "" {
		response.ErrorFrom(c, service.ErrAIModelRequired)
		c.Abort()
		return
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.N > 4 {
		response.BadRequest(c, "n must be between 1 and 4")
		c.Abort()
		return
	}

	groupRequest := parseUserAIGroupRequest(req.GroupID, req.GroupName, req.Group)
	group, err := h.userAIService.ResolveImageRequestedGroup(c.Request.Context(), subject.UserID, groupRequest)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}
	resolvedGroupID := group.ID

	internalKey, err := h.userAIService.GetOrCreateInternalKey(c.Request.Context(), subject.UserID, &resolvedGroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}

	payload := map[string]any{
		"prompt":          req.Prompt,
		"model":           req.Model,
		"size":            req.Size,
		"n":               req.N,
		"response_format": "url",
	}
	cleanBody, err := json.Marshal(payload)
	if err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}

	c.Set(userAIGroupIDContextKey, resolvedGroupID)
	c.Set(userAIImagePromptContextKey, req.Prompt)
	c.Set(userAIImageModelContextKey, req.Model)
	c.Set(userAIImageSizeContextKey, req.Size)
	c.Set(userAIImageCountContextKey, req.N)
	c.Set(userAIImageUserContextKey, req.Prompt)
	c.Set(userAIConversationIDContextKey, parseOptionalInt64Value(req.ConversationID))
	c.Request.URL.Path = "/v1/images/generations"
	c.Request.Header.Set("Authorization", "Bearer "+internalKey.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Del("x-api-key")
	c.Request.Header.Del("x-goog-api-key")
	c.Request.Body = io.NopCloser(bytes.NewReader(cleanBody))
	c.Request.ContentLength = int64(len(cleanBody))
	c.Next()
}

func (h *UserAIHandler) PrepareImageEditsProxy(c *gin.Context) {
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

	var req userAIImageEditRequest
	if err := json.Unmarshal(body, &req); err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Model = strings.TrimSpace(req.Model)
	req.Size = normalizeUserAIImageSize(req.Size)
	if req.Prompt == "" {
		response.ErrorFrom(c, service.ErrAIImageRequired)
		c.Abort()
		return
	}
	if req.Model == "" {
		response.ErrorFrom(c, service.ErrAIModelRequired)
		c.Abort()
		return
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.N > 4 {
		response.BadRequest(c, "n must be between 1 and 4")
		c.Abort()
		return
	}

	relativeImageURLs, err := validateUserAIEditImageURLs(subject.UserID, req.ImageURLs)
	if err != nil {
		response.BadRequest(c, err.Error())
		c.Abort()
		return
	}
	baseURL := userAIRequestBaseURL(c)
	if strings.TrimSpace(baseURL) == "" {
		response.BadRequest(c, "Unable to resolve site URL")
		c.Abort()
		return
	}

	groupRequest := parseUserAIGroupRequest(req.GroupID, req.GroupName, req.Group)
	group, err := h.userAIService.ResolveImageRequestedGroup(c.Request.Context(), subject.UserID, groupRequest)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}
	resolvedGroupID := group.ID

	internalKey, err := h.userAIService.GetOrCreateInternalKey(c.Request.Context(), subject.UserID, &resolvedGroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}

	images := make([]map[string]string, 0, len(relativeImageURLs))
	baseURL = strings.TrimRight(baseURL, "/")
	for _, imageURL := range relativeImageURLs {
		images = append(images, map[string]string{
			"image_url": baseURL + imageURL,
		})
	}
	payload := map[string]any{
		"prompt":          req.Prompt,
		"model":           req.Model,
		"size":            req.Size,
		"n":               req.N,
		"response_format": "url",
		"images":          images,
	}
	cleanBody, err := json.Marshal(payload)
	if err != nil {
		response.BadRequest(c, "Invalid request body")
		c.Abort()
		return
	}

	c.Set(userAIGroupIDContextKey, resolvedGroupID)
	c.Set(userAIImagePromptContextKey, req.Prompt)
	c.Set(userAIImageModelContextKey, req.Model)
	c.Set(userAIImageSizeContextKey, req.Size)
	c.Set(userAIImageCountContextKey, req.N)
	c.Set(userAIImageUserContextKey, userAIImageEditUserContent(req.Prompt, relativeImageURLs))
	c.Set(userAIConversationIDContextKey, parseOptionalInt64Value(req.ConversationID))
	c.Request.URL.Path = "/v1/images/edits"
	c.Request.Header.Set("Authorization", "Bearer "+internalKey.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Del("x-api-key")
	c.Request.Header.Del("x-goog-api-key")
	c.Request.Body = io.NopCloser(bytes.NewReader(cleanBody))
	c.Request.ContentLength = int64(len(cleanBody))
	c.Next()
}

func (h *UserAIHandler) ImageGenerations(c *gin.Context) {
	originalWriter := c.Writer
	capture := newUserAIImageBufferResponseWriter(originalWriter)
	c.Writer = capture
	defer func() {
		c.Writer = originalWriter
	}()

	h.openAIGateway.Images(c)

	status := capture.Status()
	if status < http.StatusOK || status >= http.StatusBadRequest {
		writeUserAIImageBufferedResponse(c, originalWriter, capture, capture.body.Bytes())
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		writeUserAIImageBufferedResponse(c, originalWriter, capture, capture.body.Bytes())
		return
	}

	body := bytes.TrimSpace(capture.body.Bytes())
	if len(body) == 0 || !gjson.ValidBytes(body) {
		writeUserAIImageBufferedResponse(c, originalWriter, capture, capture.body.Bytes())
		return
	}

	images := extractUserAIImageResults(body)
	if len(images) == 0 {
		writeUserAIImageBufferedResponse(c, originalWriter, capture, capture.body.Bytes())
		return
	}
	responseBody := capture.body.Bytes()
	images = h.persistGeneratedImageHistoryAssets(subject.UserID, images)
	if rewrittenBody, ok := rewriteUserAIImageResponseURLs(body, images); ok {
		responseBody = rewrittenBody
	}

	groupID, _ := getContextInt64Value(c, userAIGroupIDContextKey)
	conversationID, _ := getContextInt64Value(c, userAIConversationIDContextKey)
	prompt, _ := c.Get(userAIImagePromptContextKey)
	model, _ := c.Get(userAIImageModelContextKey)
	size, _ := c.Get(userAIImageSizeContextKey)
	n, _ := c.Get(userAIImageCountContextKey)
	promptText := stringFromAny(prompt)
	modelText := stringFromAny(model)
	userContent := promptText
	if rawUserContent := userContentStringFromContext(c, userAIImageUserContextKey); rawUserContent != "" {
		userContent = rawUserContent
	}
	_, _ = h.userAIService.SaveImageGeneration(c.Request.Context(), service.ImageGenerationHistoryCreateInput{
		UserID:  subject.UserID,
		GroupID: optionalPositiveInt64(groupID),
		Prompt:  promptText,
		Model:   modelText,
		Size:    stringFromAny(size),
		N:       maxInt(len(images), parseOptionalInt(n)),
		Images:  images,
	})
	if conversationID > 0 {
		_ = h.userAIService.SaveChatTurn(
			c.Request.Context(),
			subject.UserID,
			conversationID,
			optionalPositiveInt64(groupID),
			modelText,
			userContent,
			userAIImageAssistantContent(images),
		)
	}
	writeUserAIImageBufferedResponse(c, originalWriter, capture, responseBody)
}

type userAIImageBufferResponseWriter struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	size    int
	written bool
}

func newUserAIImageBufferResponseWriter(base gin.ResponseWriter) *userAIImageBufferResponseWriter {
	return &userAIImageBufferResponseWriter{
		ResponseWriter: base,
		status:         http.StatusOK,
		size:           -1,
	}
}

func (w *userAIImageBufferResponseWriter) WriteHeader(code int) {
	if code > 0 && !w.written {
		w.status = code
	}
}

func (w *userAIImageBufferResponseWriter) WriteHeaderNow() {
	if !w.written {
		w.written = true
		w.size = 0
	}
}

func (w *userAIImageBufferResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.body.Write(data)
	w.size += n
	return n, err
}

func (w *userAIImageBufferResponseWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	n, err := w.body.WriteString(data)
	w.size += n
	return n, err
}

func (w *userAIImageBufferResponseWriter) Status() int {
	return w.status
}

func (w *userAIImageBufferResponseWriter) Size() int {
	return w.size
}

func (w *userAIImageBufferResponseWriter) Written() bool {
	return w.written
}

func (w *userAIImageBufferResponseWriter) Flush() {
	w.WriteHeaderNow()
}

func writeUserAIImageBufferedResponse(c *gin.Context, writer gin.ResponseWriter, capture *userAIImageBufferResponseWriter, body []byte) {
	c.Writer = writer
	if len(body) > 0 {
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
	} else {
		c.Writer.Header().Del("Content-Length")
	}
	c.Writer.WriteHeader(capture.Status())
	if len(body) > 0 {
		_, _ = c.Writer.Write(body)
		return
	}
	c.Writer.WriteHeaderNow()
}

func (h *UserAIHandler) ListImageHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 50 {
		pageSize = 50
	}
	items, pageResult, err := h.userAIService.ListImageGenerationHistory(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, page, pageSize)
}

func normalizeUserAIImageSize(size string) string {
	switch strings.TrimSpace(size) {
	case "", "1:1", "square", "1024x1024":
		return "1024x1024"
	case "16:9", "landscape", "2048x1152":
		return "2048x1152"
	case "9:16", "portrait", "1152x2048":
		return "1152x2048"
	default:
		return strings.TrimSpace(size)
	}
}

func parseUserAIGroupRequest(groupIDValue, groupNameValue, groupValue any) service.AIGroupRequest {
	request := service.AIGroupRequest{
		GroupID:   parseOptionalInt64(groupIDValue),
		GroupName: strings.TrimSpace(stringFromAny(groupNameValue)),
	}
	if request.GroupID != nil || request.GroupName != "" {
		return request
	}
	if groupID := parseOptionalInt64(groupValue); groupID != nil {
		request.GroupID = groupID
		return request
	}
	request.GroupName = strings.TrimSpace(stringFromAny(groupValue))
	return request
}

func validateUserAIEditImageURLs(userID int64, imageURLs []string) ([]string, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user")
	}
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("image_urls is required")
	}
	if len(imageURLs) > 4 {
		return nil, fmt.Errorf("image_urls must contain at most 4 images")
	}

	result := make([]string, 0, len(imageURLs))
	seen := make(map[string]struct{}, len(imageURLs))
	for _, raw := range imageURLs {
		imageURL := strings.TrimSpace(raw)
		if imageURL == "" {
			continue
		}
		lower := strings.ToLower(imageURL)
		if strings.HasPrefix(lower, "data:") {
			return nil, fmt.Errorf("data image URLs are not allowed")
		}
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(imageURL, "//") {
			return nil, fmt.Errorf("external image URLs are not allowed")
		}
		matches := userAIEditImageURLPattern.FindStringSubmatch(imageURL)
		if len(matches) != 2 {
			return nil, fmt.Errorf("image_urls must reference uploaded site images")
		}
		ownerID, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || ownerID != userID {
			return nil, fmt.Errorf("image_urls cannot reference another user")
		}
		if _, exists := seen[imageURL]; exists {
			continue
		}
		seen[imageURL] = struct{}{}
		result = append(result, imageURL)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("image_urls is required")
	}
	return result, nil
}

func userAIImageEditUserContent(prompt string, imageURLs []string) string {
	parts := make([]map[string]any, 0, len(imageURLs)+1)
	if prompt = strings.TrimSpace(prompt); prompt != "" {
		parts = append(parts, map[string]any{
			"type": "text",
			"text": prompt,
		})
	}
	for _, imageURL := range imageURLs {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url": imageURL,
			},
		})
	}
	if len(parts) == 0 {
		return ""
	}
	body, err := json.Marshal(parts)
	if err != nil {
		return strings.TrimSpace(prompt)
	}
	return string(body)
}

func userContentStringFromContext(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringFromAny(value))
}

func extractUserAIImageResults(body []byte) []string {
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() {
		return nil
	}
	images := make([]string, 0, len(data.Array()))
	for _, item := range data.Array() {
		if url := strings.TrimSpace(item.Get("url").String()); url != "" {
			images = append(images, url)
			continue
		}
		if b64 := strings.TrimSpace(item.Get("b64_json").String()); b64 != "" {
			images = append(images, "data:image/png;base64,"+b64)
		}
	}
	return images
}

func (h *UserAIHandler) persistGeneratedImageHistoryAssets(userID int64, images []string) []string {
	if userID <= 0 || len(images) == 0 {
		return images
	}
	out := make([]string, 0, len(images))
	for _, image := range images {
		if stored, ok := h.storeGeneratedImageDataURL(userID, image); ok {
			out = append(out, stored)
			continue
		}
		if stored, ok := h.storeGeneratedImageRemoteURL(userID, image); ok {
			out = append(out, stored)
			continue
		}
		out = append(out, image)
	}
	return out
}

func (h *UserAIHandler) storeGeneratedImageDataURL(userID int64, raw string) (string, bool) {
	contentType, data, ok := decodeUserAIImageDataURL(raw)
	if !ok || len(data) == 0 || int64(len(data)) > h.uploadMaxFileSize {
		return "", false
	}

	ext, exists := allowedUserAIImageTypes[contentType]
	if !exists {
		detected := detectUserAIImageContentType(data)
		var detectedOK bool
		ext, detectedOK = allowedUserAIImageTypes[detected]
		if !detectedOK {
			return "", false
		}
		contentType = detected
	}
	if detectUserAIImageContentType(data) != contentType {
		return "", false
	}

	filename, err := randomUserAIImageFilename(ext)
	if err != nil {
		return "", false
	}

	userPart := strconv.FormatInt(userID, 10)
	dir := filepath.Join(h.uploadRoot, userPart, "generated")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", false
	}
	return h.uploadPublicRoot + "/" + userPart + "/generated/" + filename, true
}

func (h *UserAIHandler) storeGeneratedImageRemoteURL(userID int64, raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, h.uploadPublicRoot+"/") {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", false
	}
	if isBlockedUserAIImageHost(parsed.Hostname()) {
		return "", false
	}
	if err := urlvalidator.ValidateResolvedIP(parsed.Hostname()); err != nil {
		return "", false
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, trimmed, nil)
	if err != nil {
		return "", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return "", false
	}

	limited := io.LimitReader(resp.Body, h.uploadMaxFileSize+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) == 0 || int64(len(data)) > h.uploadMaxFileSize {
		return "", false
	}
	contentType := detectUserAIImageContentType(data)
	if declared := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])); declared != "" && strings.HasPrefix(declared, "image/") {
		contentType = declared
	}
	ext, ok := allowedUserAIImageTypes[contentType]
	if !ok {
		contentType = detectUserAIImageContentType(data)
		ext, ok = allowedUserAIImageTypes[contentType]
		if !ok {
			return "", false
		}
	}
	if detectUserAIImageContentType(data) != contentType {
		return "", false
	}

	filename, err := randomUserAIImageFilename(ext)
	if err != nil {
		return "", false
	}
	userPart := strconv.FormatInt(userID, 10)
	dir := filepath.Join(h.uploadRoot, userPart, "generated")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", false
	}
	return h.uploadPublicRoot + "/" + userPart + "/generated/" + filename, true
}

func isBlockedUserAIImageHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func rewriteUserAIImageResponseURLs(body []byte, images []string) ([]byte, bool) {
	if len(images) == 0 || len(bytes.TrimSpace(body)) == 0 || !gjson.ValidBytes(body) {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	data, ok := payload["data"].([]any)
	if !ok || len(data) == 0 {
		return nil, false
	}
	changed := false
	for i, image := range images {
		if i >= len(data) {
			break
		}
		item, ok := data[i].(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(image) == "" {
			continue
		}
		item["url"] = image
		delete(item, "b64_json")
		changed = true
	}
	if !changed {
		return nil, false
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return out, true
}

func userAIImageAssistantContent(images []string) string {
	parts := make([]map[string]any, 0, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url": image,
			},
		})
	}
	if len(parts) == 0 {
		return ""
	}
	body, err := json.Marshal(parts)
	if err != nil {
		return fmt.Sprintf("%v", images)
	}
	return string(body)
}

func decodeUserAIImageDataURL(raw string) (string, []byte, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(trimmed), "data:image/") {
		return "", nil, false
	}
	meta, payload, ok := strings.Cut(trimmed, ",")
	if !ok {
		return "", nil, false
	}
	meta = strings.TrimSpace(meta)
	if !strings.HasSuffix(strings.ToLower(meta), ";base64") {
		return "", nil, false
	}
	contentType := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(meta, "data:"), ";base64"))
	if contentType == "" {
		return "", nil, false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return "", nil, false
	}
	return strings.ToLower(contentType), data, true
}

func optionalPositiveInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func parseOptionalInt(v any) int {
	if parsed := parseOptionalInt64Value(v); parsed > 0 {
		return int(parsed)
	}
	return 0
}

func maxInt(values ...int) int {
	maxValue := 0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}
