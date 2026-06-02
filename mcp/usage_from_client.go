package mcp

// TokenUsageFromClient returns usage from the last parsed API response when the client embeds *Client.
func TokenUsageFromClient(c AIClient) TokenUsage {
	if c == nil {
		return TokenUsage{}
	}
	if embedder, ok := c.(ClientEmbedder); ok {
		return embedder.BaseClient().LastUsage
	}
	return TokenUsage{}
}
