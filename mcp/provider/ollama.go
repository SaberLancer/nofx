package provider

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"nofx/mcp"
	"nofx/security"
)

const (
	DefaultOllamaBaseURL     = "http://localhost:11434/v1"
	DefaultOllamaModel       = "llama3.1"
	DefaultOllamaAPIKey      = "ollama"
	defaultOllamaTimeout     = 15 * time.Minute
	defaultOllamaMaxContext  = 8192
	defaultOllamaMaxTokens   = 1024
	defaultOllamaKeepAlive   = "30m"
	ollamaPromptTruncateNote = "[NOFX: prompt truncated for local Ollama context limit]\n\n"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderOllama, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewOllamaClientWithOptions(opts...)
	})
}

// OllamaClient uses Ollama's OpenAI-compatible API.
type OllamaClient struct {
	*mcp.Client
}

func (c *OllamaClient) BaseClient() *mcp.Client { return c.Client }

func NewOllamaClient() mcp.AIClient {
	return NewOllamaClientWithOptions()
}

func NewOllamaClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	timeout := ollamaTimeout()
	maxContext := ollamaMaxContext()
	maxTokens := ollamaMaxTokens()
	ollamaOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderOllama),
		mcp.WithModel(ollamaDefaultModel()),
		mcp.WithBaseURL(ollamaDefaultBaseURL()),
		mcp.WithTimeout(timeout),
		mcp.WithHTTPClient(security.LocalHTTPClient(timeout)),
		mcp.WithMaxRetries(1),
		mcp.WithMaxTokens(maxTokens),
		mcp.WithMaxContext(maxContext),
	}

	allOpts := append(ollamaOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)
	ollamaClient := &OllamaClient{Client: baseClient}
	baseClient.Hooks = ollamaClient
	return ollamaClient
}

func (c *OllamaClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = DefaultOllamaAPIKey
	}
	c.APIKey = apiKey

	if customURL != "" {
		c.BaseURL = customURL
	} else if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = ollamaDefaultBaseURL()
	}
	c.Log.Infof("🔧 [MCP] Ollama BaseURL: %s", c.BaseURL)

	if customModel != "" {
		c.Model = customModel
	} else if strings.TrimSpace(c.Model) == "" {
		c.Model = ollamaDefaultModel()
	}
	c.Log.Infof("🔧 [MCP] Ollama Model: %s", c.Model)
	c.logLocalModelHints(c.Model)
}

func (c *OllamaClient) logLocalModelHints(model string) {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "14b") || strings.Contains(lower, "32b") || strings.Contains(lower, "70b") {
		c.Log.Warnf("⚠️  [MCP] Ollama model %s is large; trading prompts may be slow. Consider qwen2.5:7b / llama3.1:8b for faster cycles.", model)
	}
	c.Log.Infof("🔧 [MCP] Ollama limits: timeout=%v max_context=%d max_tokens=%d", ollamaTimeout(), ollamaMaxContext(), ollamaMaxTokens())
}

func (c *OllamaClient) SetAuthHeader(reqHeaders http.Header) {
	c.Client.SetAuthHeader(reqHeaders)
}

func (c *OllamaClient) BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	userPrompt = truncateOllamaUserPrompt(systemPrompt, userPrompt, ollamaMaxContext(), c.MaxTokens)
	body := c.Client.BuildMCPRequestBody(systemPrompt, userPrompt)
	return applyOllamaRequestOptions(body, c.MaxTokens, ollamaMaxContext())
}

func (c *OllamaClient) BuildRequestBodyFromRequest(req *mcp.Request) map[string]any {
	body := c.Client.BuildRequestBodyFromRequest(req)
	return applyOllamaRequestOptions(body, c.MaxTokens, ollamaMaxContext())
}

func (c *OllamaClient) ParseMCPResponse(body []byte) (string, error) {
	r, err := c.Client.ParseMCPResponseFull(body)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(r.Content) != "" {
		return r.Content, nil
	}
	if strings.TrimSpace(r.ReasoningContent) != "" {
		return r.ReasoningContent, nil
	}
	return "", nil
}

func applyOllamaRequestOptions(body map[string]any, maxTokens, maxContext int) map[string]any {
	if body == nil {
		body = map[string]any{}
	}
	body["keep_alive"] = ollamaKeepAlive()
	body["options"] = map[string]any{
		"num_ctx":     maxContext,
		"num_predict": maxTokens,
	}
	return body
}

func truncateOllamaUserPrompt(systemPrompt, userPrompt string, maxContext, maxTokens int) string {
	if maxContext <= 0 {
		return userPrompt
	}
	budgetTokens := maxContext - maxTokens
	if budgetTokens <= 0 {
		budgetTokens = maxContext / 2
	}
	budgetChars := budgetTokens * 3
	systemChars := utf8.RuneCountInString(systemPrompt)
	remain := budgetChars - systemChars - utf8.RuneCountInString(ollamaPromptTruncateNote)
	if remain <= 0 {
		remain = budgetChars / 2
	}
	userRunes := []rune(userPrompt)
	if len(userRunes) <= remain {
		return userPrompt
	}
	tail := string(userRunes[len(userRunes)-remain:])
	return ollamaPromptTruncateNote + tail
}

func ollamaDefaultBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultOllamaBaseURL
}

func ollamaDefaultModel() string {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_MODEL")); v != "" {
		return v
	}
	return DefaultOllamaModel
}

func ollamaTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_TIMEOUT_SECONDS")); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultOllamaTimeout
}

func ollamaMaxContext() int {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_MAX_CONTEXT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultOllamaMaxContext
}

func ollamaMaxTokens() int {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultOllamaMaxTokens
}

func ollamaKeepAlive() string {
	if v := strings.TrimSpace(os.Getenv("OLLAMA_KEEP_ALIVE")); v != "" {
		return v
	}
	return defaultOllamaKeepAlive
}
