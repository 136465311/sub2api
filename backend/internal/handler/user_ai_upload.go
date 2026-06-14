package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

const (
	userAIUploadRoot        = "/app/data/uploads/user_ai"
	userAIUploadPublicRoot  = "/uploads/user_ai"
	userAIUploadMaxFileSize = 20 << 20
)

const UserAIUploadRequestBodyLimit = userAIUploadMaxFileSize + (1 << 20)

var allowedUserAIImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type userAIUploadResponse struct {
	ImageURL string `json:"image_url"`
}

func (h *UserAIHandler) UploadImage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if _, ok := extractMaxBytesError(err); ok {
			response.Error(c, http.StatusRequestEntityTooLarge, "Image must be 20MB or smaller")
			return
		}
		response.BadRequest(c, "Image file is required")
		return
	}
	defer func() { _ = file.Close() }()

	result, err := h.saveUploadedUserAIImage(subject.UserID, file, header)
	if err != nil {
		if errors.Is(err, errUserAIUploadTooLarge) {
			response.Error(c, http.StatusRequestEntityTooLarge, "Image must be 20MB or smaller")
			return
		}
		if errors.Is(err, errUserAIUploadType) {
			response.BadRequest(c, "Only JPEG, PNG, WebP, and GIF images are allowed")
			return
		}
		response.InternalError(c, "Failed to upload image")
		return
	}

	response.Success(c, result)
}

var (
	errUserAIUploadTooLarge = errors.New("user ai upload too large")
	errUserAIUploadType     = errors.New("user ai upload type not allowed")
)

func (h *UserAIHandler) saveUploadedUserAIImage(userID int64, file multipart.File, header *multipart.FileHeader) (*userAIUploadResponse, error) {
	if userID <= 0 {
		return nil, errUserAIUploadType
	}
	if header != nil && header.Size > h.uploadMaxFileSize {
		return nil, errUserAIUploadTooLarge
	}

	limited := io.LimitReader(file, h.uploadMaxFileSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > h.uploadMaxFileSize {
		return nil, errUserAIUploadTooLarge
	}
	if len(data) == 0 {
		return nil, errUserAIUploadType
	}

	contentType := detectUserAIImageContentType(data)
	ext, ok := allowedUserAIImageTypes[contentType]
	if !ok {
		return nil, errUserAIUploadType
	}
	if header != nil && header.Header != nil {
		declared := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
		if declared != "" && declared != contentType {
			return nil, errUserAIUploadType
		}
	}

	filename, err := randomUserAIImageFilename(ext)
	if err != nil {
		return nil, err
	}

	userPart := strconv.FormatInt(userID, 10)
	dir := filepath.Join(h.uploadRoot, userPart)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}

	return &userAIUploadResponse{
		ImageURL: h.uploadPublicRoot + "/" + userPart + "/" + filename,
	}, nil
}

func detectUserAIImageContentType(data []byte) string {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return http.DetectContentType(data)
}

func randomUserAIImageFilename(ext string) (string, error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if _, ok := map[string]struct{}{".jpg": {}, ".png": {}, ".webp": {}, ".gif": {}}[ext]; !ok {
		return "", fmt.Errorf("invalid image extension")
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]) + ext, nil
}
