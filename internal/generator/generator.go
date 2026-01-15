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

func (g *Generator) Generate(ctx context.Context, description string, tlds []string) ([]string, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := buildPrompt(description, tlds)

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

func buildPrompt(description string, tlds []string) string {
	return fmt.Sprintf(`Generate 50 creative domain name suggestions based on this project description:

"%s"

TLDs to use: %s

Guidelines:
- IMPORTANT: Generate at least 60%% of suggestions as .com domains since they're most desirable
- Use unusual, invented, or uncommon words that are more likely to be available as .com
- Mix naming strategies: invented words, creative misspellings, word mashups, prefixes/suffixes, metaphors
- Keep names short (ideally 6-10 characters before TLD)
- Make them memorable, brandable, and easy to spell
- Avoid hyphens and numbers
- Be creative - capture the essence and vibe, don't just use literal words from the description
- Think like a startup founder looking for an available .com - get creative with spelling and word combinations

Output format: Return ONLY a JSON array of domain names, nothing else. Example:
["brandname.com", "coolstartup.io", "myproject.ai"]`, description, strings.Join(tlds, ", "))
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
