package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/security"
	"nofx/store"
	"nofx/wallet"

	"github.com/gin-gonic/gin"
)

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig Safe model configuration structure (does not contain sensitive information)
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	HasAPIKey       bool   `json:"has_api_key"`
	CustomAPIURL    string `json:"customApiUrl"`    // Custom API URL (usually not sensitive)
	CustomModelName string `json:"customModelName"` // Custom model name (not sensitive)
	WalletAddress   string `json:"walletAddress,omitempty"`
	BalanceUSDC     string `json:"balanceUsdc,omitempty"`
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
		Name            string `json:"name"`
	} `json:"models"`
}

type CreateModelConfigRequest struct {
	Provider        string `json:"provider" binding:"required"`
	Name            string `json:"name"`
	APIKey          string `json:"api_key"`
	CustomAPIURL    string `json:"custom_api_url"`
	CustomModelName string `json:"custom_model_name"`
	Enabled         *bool  `json:"enabled"`
}

// handleGetModelConfigs Get AI model configurations
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	logger.Infof("🔍 Querying AI model configs for user %s", userID)
	models, err := s.store.AIModel().List(userID)
	if err != nil {
		logger.Infof("❌ Failed to get AI model configs: %v", err)
		SafeInternalError(c, "Failed to get AI model configs", err)
		return
	}

	// If no models in database, return default models
	if len(models) == 0 {
		logger.Infof("⚠️ No AI models in database, returning defaults")
		defaultModels := []SafeModelConfig{
			{ID: "deepseek", Name: "DeepSeek AI", Provider: "deepseek", Enabled: false, HasAPIKey: false},
			{ID: "qwen", Name: "Qwen AI", Provider: "qwen", Enabled: false, HasAPIKey: false},
			{ID: "openai", Name: "OpenAI", Provider: "openai", Enabled: false, HasAPIKey: false},
			{ID: "claude", Name: "Claude AI", Provider: "claude", Enabled: false, HasAPIKey: false},
			{ID: "gemini", Name: "Gemini AI", Provider: "gemini", Enabled: false, HasAPIKey: false},
			{ID: "grok", Name: "Grok AI", Provider: "grok", Enabled: false, HasAPIKey: false},
			{ID: "kimi", Name: "Kimi AI", Provider: "kimi", Enabled: false, HasAPIKey: false},
			{ID: "minimax", Name: "MiniMax AI", Provider: "minimax", Enabled: false, HasAPIKey: false},
		}
		c.JSON(http.StatusOK, defaultModels)
		return
	}

	logger.Infof("✅ Found %d AI model configs", len(models))

	// Convert to safe response structure, remove sensitive information
	safeModels := make([]SafeModelConfig, 0, len(models))
	for _, model := range models {
		if !store.IsVisibleAIModel(model) {
			continue
		}
		safeModel := SafeModelConfig{
			ID:              model.ID,
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			HasAPIKey:       model.APIKey != "",
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}

		if model.Provider == "claw402" {
			if privateKey := strings.TrimSpace(model.APIKey.String()); privateKey != "" {
				if walletAddress, addrErr := walletAddressFromPrivateKey(privateKey); addrErr == nil {
					safeModel.WalletAddress = walletAddress
					safeModel.BalanceUSDC = wallet.QueryUSDCBalanceStr(walletAddress)
				} else {
					logger.Warnf("⚠️ Failed to derive claw402 wallet address for model %s: %v", model.ID, addrErr)
				}
			}
		}

		safeModels = append(safeModels, safeModel)
	}

	if len(safeModels) == 0 {
		logger.Infof("⚠️ No visible AI models in database, returning defaults")
		defaultModels := []SafeModelConfig{
			{ID: "deepseek", Name: "DeepSeek AI", Provider: "deepseek", Enabled: false, HasAPIKey: false},
			{ID: "qwen", Name: "Qwen AI", Provider: "qwen", Enabled: false, HasAPIKey: false},
			{ID: "openai", Name: "OpenAI", Provider: "openai", Enabled: false, HasAPIKey: false},
			{ID: "claude", Name: "Claude AI", Provider: "claude", Enabled: false, HasAPIKey: false},
			{ID: "gemini", Name: "Gemini AI", Provider: "gemini", Enabled: false, HasAPIKey: false},
			{ID: "grok", Name: "Grok AI", Provider: "grok", Enabled: false, HasAPIKey: false},
			{ID: "kimi", Name: "Kimi AI", Provider: "kimi", Enabled: false, HasAPIKey: false},
			{ID: "minimax", Name: "MiniMax AI", Provider: "minimax", Enabled: false, HasAPIKey: false},
		}
		c.JSON(http.StatusOK, defaultModels)
		return
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs Update AI model configurations (supports both encrypted and plain text based on config)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req UpdateModelConfigRequest

	// Check if transport encryption is enabled
	if !cfg.TransportEncryption {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			logger.Infof("❌ Failed to parse plain JSON request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		logger.Infof("📝 Received plain text model config (UserID: %s)", userID)
	} else {
		// Transport encryption enabled, require encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			logger.Infof("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format, encrypted transmission required"})
			return
		}

		// Verify encrypted data
		if encryptedPayload.WrappedKey == "" {
			logger.Infof("❌ Detected unencrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			logger.Infof("❌ Failed to decrypt model config (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			logger.Infof("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		logger.Infof("🔓 Decrypted model config data (UserID: %s)", userID)
	}

	// Update each model's configuration and track traders that need reload
	tradersToReload := make(map[string]bool)
	for modelID, modelData := range req.Models {
		// SSRF protection: validate custom_api_url before storing
		if modelData.CustomAPIURL != "" {
			cleanURL := strings.TrimSuffix(modelData.CustomAPIURL, "#")
			var err error
			if strings.EqualFold(modelID, "ollama") {
				err = security.ValidateLocalServiceURL(cleanURL)
			} else {
				err = security.ValidateURL(cleanURL)
			}
			if err != nil {
				logger.Warnf("Invalid custom_api_url for model %s: %v", modelID, err)
				if strings.EqualFold(modelID, "ollama") {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom_api_url for model %s: URL must be a valid http(s) endpoint (localhost allowed for Ollama)", modelID)})
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom_api_url for model %s: URL must be a valid HTTPS endpoint", modelID)})
				}
				return
			}
		}

		// Find traders using this AI model BEFORE updating
		traders, _ := s.store.Trader().ListByAIModelID(userID, modelID)
		for _, t := range traders {
			tradersToReload[t.ID] = true
		}

		err := s.store.AIModel().UpdateWithName(
			userID, modelID, modelData.Name, modelData.Enabled,
			modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName,
		)
		if err != nil {
			SafeInternalError(c, fmt.Sprintf("Update model %s", modelID), err)
			return
		}
	}

	// Remove affected traders from memory BEFORE reloading to pick up new config
	for traderID := range tradersToReload {
		logger.Infof("🔄 Removing trader %s from memory to reload with new AI model config", traderID)
		s.traderManager.RemoveTrader(traderID)
	}

	// Reload all traders for this user to make new config take effect immediately
	err = s.traderManager.LoadUserTradersFromStore(s.store, userID)
	if err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here since model config was successfully updated to database
	}

	logger.Infof("✓ AI model config updated: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "Model configuration updated"})
}

// handleCreateModelConfig creates a new AI model binding (allows multiple per provider).
func (s *Server) handleCreateModelConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	cfg := config.Get()

	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	var req CreateModelConfigRequest
	if !cfg.TransportEncryption {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
	} else {
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil || encryptedPayload.WrappedKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Encrypted transmission is required"})
			return
		}
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	customAPIURL := strings.TrimSpace(req.CustomAPIURL)
	if customAPIURL != "" {
		cleanURL := strings.TrimSuffix(customAPIURL, "#")
		var urlErr error
		if strings.EqualFold(provider, "ollama") {
			urlErr = security.ValidateLocalServiceURL(cleanURL)
		} else {
			urlErr = security.ValidateURL(cleanURL)
		}
		if urlErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid custom_api_url: %v", urlErr)})
			return
		}
		customAPIURL = cleanURL
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	model, err := s.store.AIModel().CreateDedicated(
		userID, provider, req.Name, req.APIKey, customAPIURL, strings.TrimSpace(req.CustomModelName), enabled,
	)
	if err != nil {
		if strings.Contains(err.Error(), "api_key is required") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		SafeInternalError(c, "Create model config", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":              model.ID,
		"name":            model.Name,
		"provider":        model.Provider,
		"enabled":         model.Enabled,
		"customApiUrl":    model.CustomAPIURL,
		"customModelName": model.CustomModelName,
		"has_api_key":     model.APIKey != "",
	})
}

// handleDeleteModelConfig removes an AI model binding by id.
func (s *Server) handleDeleteModelConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	modelID := strings.TrimSpace(c.Param("id"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model id is required"})
		return
	}

	traders, _ := s.store.Trader().ListByAIModelID(userID, modelID)
	if len(traders) > 0 {
		names := make([]string, 0, len(traders))
		for _, t := range traders {
			if t != nil {
				names = append(names, t.Name)
			}
		}
		c.JSON(http.StatusConflict, gin.H{
			"error":   "model is bound to traders",
			"traders": names,
		})
		return
	}

	if err := s.store.AIModel().Delete(userID, modelID); err != nil {
		SafeInternalError(c, "Delete model config", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deleted", "id": modelID})
}

// handleGetSupportedModels Get list of AI models supported by the system
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// Return static list of supported AI models with default versions
	supportedModels := []map[string]interface{}{
		{"id": "deepseek", "name": "DeepSeek", "provider": "deepseek", "defaultModel": "deepseek-chat"},
		{"id": "qwen", "name": "Qwen", "provider": "qwen", "defaultModel": "qwen3-max"},
		{"id": "openai", "name": "OpenAI", "provider": "openai", "defaultModel": "gpt-5.1"},
		{"id": "claude", "name": "Claude", "provider": "claude", "defaultModel": "claude-opus-4-6"},
		{"id": "gemini", "name": "Google Gemini", "provider": "gemini", "defaultModel": "gemini-3-pro-preview"},
		{"id": "grok", "name": "Grok (xAI)", "provider": "grok", "defaultModel": "grok-3-latest"},
		{"id": "kimi", "name": "Kimi (Moonshot)", "provider": "kimi", "defaultModel": "moonshot-v1-auto"},
		{"id": "minimax", "name": "MiniMax", "provider": "minimax", "defaultModel": "MiniMax-M2.7"},
		{"id": "blockrun-base", "name": "BlockRun (Base Wallet)", "provider": "blockrun-base", "defaultModel": "auto"},
		{"id": "blockrun-sol", "name": "BlockRun (Solana Wallet)", "provider": "blockrun-sol", "defaultModel": "auto"},
		{"id": "claw402", "name": "Claw402 (Base USDC)", "provider": "claw402", "defaultModel": "deepseek-v4-flash"},
		{"id": "ollama", "name": "Ollama (Local)", "provider": "ollama", "defaultModel": "llama3.1"},
	}

	c.JSON(http.StatusOK, supportedModels)
}
