package handler

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUploadedUserAIImage(t *testing.T) {
	root := t.TempDir()
	h := &UserAIHandler{
		uploadRoot:        root,
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}

	file, header := multipartImageFile(t, "chart.png", "image/png", tinyPNG())
	result, err := h.saveUploadedUserAIImage(42, file, header)
	if err != nil {
		t.Fatalf("saveUploadedUserAIImage returned error: %v", err)
	}
	if result == nil || !strings.HasPrefix(result.ImageURL, "/uploads/user_ai/42/") || !strings.HasSuffix(result.ImageURL, ".png") {
		t.Fatalf("unexpected image_url: %#v", result)
	}
	filename := strings.TrimPrefix(result.ImageURL, "/uploads/user_ai/42/")
	if strings.Contains(filename, "chart") || strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
		t.Fatalf("filename is not a safe random basename: %q", filename)
	}
	if _, err := os.Stat(filepath.Join(root, "42", filename)); err != nil {
		t.Fatalf("uploaded file not saved: %v", err)
	}
}

func TestSaveUploadedUserAIImageRejectsInvalidType(t *testing.T) {
	h := &UserAIHandler{
		uploadRoot:        t.TempDir(),
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: userAIUploadMaxFileSize,
	}

	file, header := multipartImageFile(t, "note.txt", "text/plain", []byte("not an image"))
	_, err := h.saveUploadedUserAIImage(42, file, header)
	if !errors.Is(err, errUserAIUploadType) {
		t.Fatalf("expected type error, got %v", err)
	}
}

func TestSaveUploadedUserAIImageRejectsOversize(t *testing.T) {
	h := &UserAIHandler{
		uploadRoot:        t.TempDir(),
		uploadPublicRoot:  userAIUploadPublicRoot,
		uploadMaxFileSize: 8,
	}

	file, header := multipartImageFile(t, "chart.png", "image/png", tinyPNG())
	_, err := h.saveUploadedUserAIImage(42, file, header)
	if !errors.Is(err, errUserAIUploadTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestDetectUserAIImageContentTypeDetectsWebP(t *testing.T) {
	data := []byte("RIFF\x01\x00\x00\x00WEBPVP8 ")
	if got := detectUserAIImageContentType(data); got != "image/webp" {
		t.Fatalf("content type = %q, want image/webp", got)
	}
}

func multipartImageFile(t *testing.T, filename, contentType string, data []byte) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	header := &multipart.FileHeader{
		Filename: filename,
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", contentType)
	return &memoryMultipartFile{Reader: bytes.NewReader(data)}, header
}

type memoryMultipartFile struct {
	*bytes.Reader
}

func (f *memoryMultipartFile) Close() error {
	return nil
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89,
	}
}
