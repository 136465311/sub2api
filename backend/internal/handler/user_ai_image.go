package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	userAIImagePromptContextKey = "_user_ai_image_prompt"
	userAIImageModelContextKey  = "_user_ai_image_model"
	userAIImageSizeContextKey   = "_user_ai_image_size"
	userAIImageCountContextKey  = "_user_ai_image_count"
)

type userAIImageGenerationRequest struct {
	Prompt  string `json:"prompt"`
	Model   string `json:"model"`
	Size    string `json:"size"`
	N       int    `json:"n"`
	GroupID *int64 `json:"group_id"`
}

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

	group, err := h.userAIService.ResolveImageGroup(c.Request.Context(), subject.UserID, req.GroupID)
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
	c.Request.URL.Path = "/v1/images/generations"
	c.Request.Header.Set("Authorization", "Bearer "+internalKey.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Del("x-api-key")
	c.Request.Header.Del("x-goog-api-key")
	c.Request.Body = io.NopCloser(bytes.NewReader(cleanBody))
	c.Request.ContentLength = int64(len(cleanBody))
	c.Next()
}

func (h *UserAIHandler) ImageGenerations(c *gin.Context) {
	capture := &captureResponseWriter{ResponseWriter: c.Writer}
	c.Writer = capture

	h.openAIGateway.Images(c)

	status := capture.Status()
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return
	}

	body := bytes.TrimSpace(capture.body.Bytes())
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return
	}

	images := extractUserAIImageResults(body)
	if len(images) == 0 {
		return
	}
	images = h.persistGeneratedImageHistoryAssets(subject.UserID, images)

	groupID, _ := getContextInt64Value(c, userAIGroupIDContextKey)
	prompt, _ := c.Get(userAIImagePromptContextKey)
	model, _ := c.Get(userAIImageModelContextKey)
	size, _ := c.Get(userAIImageSizeContextKey)
	n, _ := c.Get(userAIImageCountContextKey)
	_, _ = h.userAIService.SaveImageGeneration(c.Request.Context(), service.ImageGenerationHistoryCreateInput{
		UserID:  subject.UserID,
		GroupID: optionalPositiveInt64(groupID),
		Prompt:  stringFromAny(prompt),
		Model:   stringFromAny(model),
		Size:    stringFromAny(size),
		N:       maxInt(len(images), parseOptionalInt(n)),
		Images:  images,
	})
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
