package store

import (
	"errors"
	"fmt"
	"nofx/crypto"
	"nofx/logger"
	"strings"
	"time"

	"gorm.io/gorm"
)

var knownProviderSlugs = map[string]bool{
	"deepseek": true, "openai": true, "qwen": true, "claude": true, "gemini": true,
	"grok": true, "kimi": true, "minimax": true, "ollama": true,
	"claw402": true, "blockrun-base": true, "blockrun-sol": true,
}

func isKnownProviderSlug(s string) bool {
	return knownProviderSlugs[strings.ToLower(strings.TrimSpace(s))]
}

func slugifyModelSegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// GenerateModelID builds a stable id for a new model config (user + provider + variant).
func GenerateModelID(userID, provider, customModelName string) string {
	userID = strings.TrimSpace(userID)
	provider = strings.ToLower(strings.TrimSpace(provider))
	base := fmt.Sprintf("%s_%s", userID, provider)
	if slug := slugifyModelSegment(customModelName); slug != "" {
		return fmt.Sprintf("%s_%s", base, slug)
	}
	return base
}

func (s *AIModelStore) ensureUniqueModelID(userID, baseID string) string {
	id := strings.TrimSpace(baseID)
	for i := 0; i < 100; i++ {
		_, err := s.Get(userID, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return id
		}
		if i == 0 {
			id = baseID + "_2"
		} else {
			id = fmt.Sprintf("%s_%d", baseID, i+2)
		}
	}
	return fmt.Sprintf("%s_%d", baseID, time.Now().Unix())
}

func defaultModelDisplayName(provider, customModelName string) string {
	provider = strings.TrimSpace(provider)
	if customModelName != "" {
		return customModelName
	}
	switch strings.ToLower(provider) {
	case "deepseek":
		return "DeepSeek AI"
	case "qwen":
		return "Qwen AI"
	default:
		if provider == "" {
			return "AI Model"
		}
		return provider + " AI"
	}
}

// CreateDedicated inserts a new AI model row (never merges with existing provider configs).
func (s *AIModelStore) CreateDedicated(userID, provider, name, apiKey, customAPIURL, customModelName string, enabled bool) (*AIModel, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(apiKey) == "" && provider != "ollama" {
		return nil, fmt.Errorf("api_key is required")
	}

	baseID := GenerateModelID(userID, provider, customModelName)
	id := s.ensureUniqueModelID(userID, baseID)

	finalName := strings.TrimSpace(name)
	if finalName == "" {
		finalName = defaultModelDisplayName(provider, customModelName)
	}

	model := &AIModel{
		ID:              id,
		UserID:          userID,
		Name:            finalName,
		Provider:        provider,
		Enabled:         enabled,
		APIKey:          crypto.EncryptedString(apiKey),
		CustomAPIURL:    customAPIURL,
		CustomModelName: customModelName,
	}
	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}
	logger.Infof("✓ Created AI model: id=%s provider=%s name=%s", id, provider, finalName)
	return model, nil
}

func (s *AIModelStore) applyModelUpdates(existing *AIModel, name string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	updates := map[string]interface{}{
		"enabled":           enabled,
		"custom_api_url":    customAPIURL,
		"custom_model_name": customModelName,
		"updated_at":        time.Now().UTC(),
	}
	if strings.TrimSpace(name) != "" {
		updates["name"] = strings.TrimSpace(name)
	}
	if apiKey != "" {
		updates["api_key"] = crypto.EncryptedString(apiKey)
	}
	return s.db.Model(existing).Updates(updates).Error
}
