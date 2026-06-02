package store

import (
	"errors"
	"fmt"
	"nofx/crypto"
	"nofx/logger"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AIModelStore AI model storage
type AIModelStore struct {
	db *gorm.DB
}

// AIModel AI model configuration
type AIModel struct {
	ID              string                 `gorm:"primaryKey" json:"id"`
	UserID          string                 `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	Name            string                 `gorm:"not null" json:"name"`
	Provider        string                 `gorm:"not null" json:"provider"`
	Enabled         bool                   `gorm:"default:false" json:"enabled"`
	APIKey          crypto.EncryptedString `gorm:"column:api_key;default:''" json:"apiKey"`
	CustomAPIURL    string                 `gorm:"column:custom_api_url;default:''" json:"customApiUrl"`
	CustomModelName string                 `gorm:"column:custom_model_name;default:''" json:"customModelName"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

func (AIModel) TableName() string { return "ai_models" }

// NewAIModelStore creates a new AIModelStore
func NewAIModelStore(db *gorm.DB) *AIModelStore {
	return &AIModelStore{db: db}
}

func (s *AIModelStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'ai_models'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&AIModel{})
}

func (s *AIModelStore) initDefaultData() error {
	// No longer pre-populate AI models - create on demand when user configures
	return nil
}

// FindOrphanClaw402 finds a claw402 model whose user_id no longer exists in the users table.
// Used to recover wallets after account reset.
func (s *AIModelStore) FindOrphanClaw402() (*AIModel, error) {
	var model AIModel
	err := s.db.Where("provider = ? AND api_key != '' AND user_id NOT IN (SELECT id FROM users)", "claw402").
		First(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// AdoptModel re-assigns an existing model to a new user.
func (s *AIModelStore) AdoptModel(modelID, newUserID string) error {
	return s.db.Model(&AIModel{}).Where("id = ?", modelID).
		Update("user_id", newUserID).Error
}

// List retrieves user's AI model list
func (s *AIModelStore) List(userID string) ([]*AIModel, error) {
	var models []*AIModel
	err := s.db.Where("user_id = ?", userID).Order("id").Find(&models).Error
	if err != nil {
		return nil, err
	}
	return models, nil
}

// Get retrieves a single AI model
func (s *AIModelStore) Get(userID, modelID string) (*AIModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	candidates := []string{}
	if userID != "" {
		candidates = append(candidates, userID)
	}
	if userID != "default" {
		candidates = append(candidates, "default")
	}
	if len(candidates) == 0 {
		candidates = append(candidates, "default")
	}

	for _, uid := range candidates {
		var model AIModel
		err := s.db.Where("user_id = ? AND id = ?", uid, modelID).First(&model).Error
		if err == nil {
			return &model, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// GetByID retrieves an AI model by ID only
func (s *AIModelStore) GetByID(modelID string) (*AIModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	var model AIModel
	err := s.db.Where("id = ?", modelID).First(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// GetDefault retrieves the default enabled AI model
func (s *AIModelStore) GetDefault(userID string) (*AIModel, error) {
	if userID == "" {
		userID = "default"
	}
	model, err := s.firstEnabledUsable(userID)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if userID != "default" {
		return s.firstEnabledUsable("default")
	}
	return nil, fmt.Errorf("please configure an available AI model in the system first")
}

func (s *AIModelStore) firstEnabledUsable(userID string) (*AIModel, error) {
	var models []AIModel
	err := s.db.Where("user_id = ? AND enabled = ? AND api_key != ''", userID, true).
		Order("updated_at DESC, id ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	for i := range models {
		if hasUsableAPIKey(models[i]) {
			return &models[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// GetAnyEnabled returns the first enabled AI model across all users.
// Used by single-user features (e.g. Telegram bot) that need any working LLM client.
func (s *AIModelStore) GetAnyEnabled() (*AIModel, error) {
	var models []AIModel
	err := s.db.Where("enabled = ?", true).
		Order("updated_at DESC, id ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	for i := range models {
		if hasUsableAPIKey(models[i]) {
			return &models[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func hasUsableAPIKey(model AIModel) bool {
	if strings.TrimSpace(string(model.APIKey)) != "" {
		return true
	}
	envKeyByProvider := map[string]string{
		"deepseek": "DEEPSEEK_API_KEY",
		"openai":   "OPENAI_API_KEY",
		"claude":   "ANTHROPIC_API_KEY",
		"gemini":   "GEMINI_API_KEY",
		"grok":     "XAI_API_KEY",
		"kimi":     "MOONSHOT_API_KEY",
		"minimax":  "MINIMAX_API_KEY",
		"qwen":     "DASHSCOPE_API_KEY",
	}
	envKey := envKeyByProvider[strings.ToLower(strings.TrimSpace(model.Provider))]
	return envKey != "" && strings.TrimSpace(os.Getenv(envKey)) != ""
}

// Update updates AI model, creates if not exists
// IMPORTANT: If apiKey is empty string, the existing API key will be preserved (not overwritten)
func (s *AIModelStore) Update(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	return s.UpdateWithName(userID, id, "", enabled, apiKey, customAPIURL, customModelName)
}

func (s *AIModelStore) UpdateWithName(userID, id, name string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	id = strings.TrimSpace(id)
	var existingModel AIModel
	err := s.db.Where("user_id = ? AND id = ?", userID, id).First(&existingModel).Error
	if err == nil {
		return s.applyModelUpdates(&existingModel, name, enabled, apiKey, customAPIURL, customModelName)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Legacy: request key is a provider slug (e.g. PUT models.deepseek) — only if unambiguous.
	provider := strings.ToLower(strings.TrimSpace(id))
	if isKnownProviderSlug(provider) {
		var matches []AIModel
		if e := s.db.Where("user_id = ? AND provider = ?", userID, provider).Find(&matches).Error; e != nil {
			return e
		}
		switch len(matches) {
		case 1:
			logger.Warnf("⚠️ Legacy provider key %q resolved to model id=%s", provider, matches[0].ID)
			return s.applyModelUpdates(&matches[0], name, enabled, apiKey, customAPIURL, customModelName)
		case 0:
			_, err := s.CreateDedicated(userID, provider, name, apiKey, customAPIURL, customModelName, enabled)
			return err
		default:
			return fmt.Errorf("multiple AI models exist for provider %q; use explicit model id (e.g. %s)", provider, matches[0].ID)
		}
	}

	// Infer provider from composite id and create (legacy single-model id patterns).
	inferred := provider
	parts := strings.Split(id, "_")
	if len(parts) >= 2 {
		inferred = strings.ToLower(parts[len(parts)-1])
	}
	if !isKnownProviderSlug(inferred) {
		return fmt.Errorf("ai model not found: id=%s", id)
	}
	_, err = s.CreateDedicated(userID, inferred, name, apiKey, customAPIURL, customModelName, enabled)
	return err
}

// Create creates an AI model
// ResolveClaw402WalletKey returns the claw402 wallet private key for a user.
// If preferredModelID is non-empty and points to a claw402 model, its key is returned first.
// Otherwise the first enabled claw402 model in the user's model list is used.
// Returns ("", nil) when no claw402 model is configured — callers should treat this as
// "no paid data routing" rather than an error.
func (s *AIModelStore) ResolveClaw402WalletKey(userID, preferredModelID string) (string, error) {
	if preferredModelID != "" {
		model, err := s.Get(userID, preferredModelID)
		if err != nil {
			return "", fmt.Errorf("failed to load selected AI model")
		}
		if model.Provider == "claw402" {
			walletKey := string(model.APIKey)
			if walletKey == "" {
				return "", fmt.Errorf("selected claw402 model is missing wallet private key")
			}
			return walletKey, nil
		}
	}

	models, err := s.List(userID)
	if err != nil {
		return "", fmt.Errorf("failed to load AI models")
	}

	for _, model := range models {
		if model == nil || model.Provider != "claw402" {
			continue
		}
		if walletKey := string(model.APIKey); walletKey != "" {
			return walletKey, nil
		}
	}

	return "", nil
}

func (s *AIModelStore) Create(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	model := &AIModel{
		ID:           id,
		UserID:       userID,
		Name:         name,
		Provider:     provider,
		Enabled:      enabled,
		APIKey:       crypto.EncryptedString(apiKey),
		CustomAPIURL: customAPIURL,
	}
	// Use FirstOrCreate to ignore if already exists
	return s.db.Where("id = ?", id).FirstOrCreate(model).Error
}

// Delete removes a user-owned AI model configuration.
func (s *AIModelStore) Delete(userID, id string) error {
	result := s.db.Where("user_id = ? AND id = ?", userID, id).Delete(&AIModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("ai model not found: id=%s, userID=%s", id, userID)
	}
	logger.Infof("🗑️ Deleted AI model: id=%s, userID=%s", id, userID)
	return nil
}
