package service

import "testing"

func TestValidateOpenAIImagesModelAcceptsCompatibleImageModels(t *testing.T) {
	tests := []string{"gpt-image", "gpt-image-2", "grok-image", "grok-image-v2"}
	for _, model := range tests {
		if err := validateOpenAIImagesModel(model); err != nil {
			t.Fatalf("validateOpenAIImagesModel(%q) returned error: %v", model, err)
		}
	}
}

func TestValidateOpenAIImagesModelRejectsTextModel(t *testing.T) {
	if err := validateOpenAIImagesModel("gpt-4o"); err == nil {
		t.Fatal("expected text model to be rejected by images endpoint")
	}
}
