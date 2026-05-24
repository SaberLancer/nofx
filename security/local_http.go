package security

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ValidateLocalServiceURL validates URLs for user-configured local services such as Ollama.
// Unlike ValidateURL, it allows localhost and private network hosts.
func ValidateLocalServiceURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("empty URL")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", scheme)
	}
	if strings.TrimSpace(parsedURL.Hostname()) == "" {
		return fmt.Errorf("empty hostname")
	}
	return nil
}

// LocalHTTPClient returns a standard HTTP client for local-only services (Ollama, LM Studio).
func LocalHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport,
	}
}
