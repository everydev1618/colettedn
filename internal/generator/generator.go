package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Generator struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Generator {
	return &Generator{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (g *Generator) Generate(ctx context.Context, keywords []string, industry, vibe string, tlds []string) ([]string, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := buildPrompt(keywords, industry, vibe, tlds)

	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 2048,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	var claudeResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if claudeResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	return parseDomains(claudeResp.Content[0].Text), nil
}

func buildPrompt(keywords []string, industry, vibe string, tlds []string) string {
	var sb strings.Builder

	sb.WriteString("Generate 30 creative domain name suggestions for a new brand/project.\n\n")
	sb.WriteString("Keywords: " + strings.Join(keywords, ", ") + "\n")

	if industry != "" {
		sb.WriteString("Industry: " + industry + "\n")
	}

	if vibe != "" {
		sb.WriteString("Vibe/Style: " + vibe + "\n")
	}

	sb.WriteString("TLDs to use: " + strings.Join(tlds, ", ") + "\n\n")

	sb.WriteString(`Guidelines:
- Mix different naming strategies: compound words, invented words, prefixes/suffixes, abbreviations
- Keep names short (ideally under 12 characters before TLD)
- Make them memorable and easy to spell
- Avoid hyphens and numbers
- Include a variety of the specified TLDs
- Be creative - don't just combine keywords literally

Output format: Return ONLY a JSON array of domain names, nothing else. Example:
["brandname.com", "coolstartup.io", "myproject.ai"]`)

	return sb.String()
}

func parseDomains(text string) []string {
	// Find JSON array in response
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")

	if start == -1 || end == -1 || end <= start {
		return nil
	}

	jsonStr := text[start : end+1]

	var domains []string
	if err := json.Unmarshal([]byte(jsonStr), &domains); err != nil {
		return nil
	}

	// Clean up domains
	var cleaned []string
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" && strings.Contains(d, ".") {
			cleaned = append(cleaned, d)
		}
	}

	return cleaned
}
