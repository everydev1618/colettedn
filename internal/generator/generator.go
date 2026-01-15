package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Generator struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Generator {
	return &Generator{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 20 * time.Second, // Per-request timeout to stay under API Gateway's 29s limit
		},
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

// GenerateCategorized generates domain names categorized by vibe
func (g *Generator) GenerateCategorized(ctx context.Context, description string, tlds []string) (map[string][]string, error) {
	return g.GenerateCategorizedWithExclusions(ctx, description, tlds, nil)
}

// GenerateCategorizedWithExclusions generates domain names, avoiding specified taken domains
func (g *Generator) GenerateCategorizedWithExclusions(ctx context.Context, description string, tlds []string, takenDomains []string) (map[string][]string, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := buildCategorizedPromptWithExclusions(description, tlds, takenDomains)

	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
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

	return parseCategorizedDomains(claudeResp.Content[0].Text), nil
}

func buildCategorizedPrompt(description string, tlds []string) string {
	return buildCategorizedPromptWithExclusions(description, tlds, nil)
}

func buildCategorizedPromptWithExclusions(description string, tlds []string, takenDomains []string) string {
	exclusionNote := ""
	if len(takenDomains) > 0 {
		// Group taken domains to show patterns, limit to avoid huge prompts
		maxToShow := 30
		if len(takenDomains) > maxToShow {
			takenDomains = takenDomains[:maxToShow]
		}
		exclusionNote = fmt.Sprintf(`

IMPORTANT - These domains are TAKEN. Avoid these exact names AND similar patterns:
%s

Since these are taken, try COMPLETELY DIFFERENT approaches:
- Use different root words and concepts
- Try more abstract/invented words
- Explore unexpected metaphors
- Consider phonetic variations that don't resemble the taken names`, strings.Join(takenDomains, ", "))
	}

	return fmt.Sprintf(`Generate creative domain name suggestions for this project, organized by personality/vibe:

PROJECT: "%s"

TLDs to use: %s%s

Generate names in 4 categories:

1. **Professional** - Clean, trustworthy, corporate feel. Think "Stripe", "Notion", "Linear", "Figma". Names that instill confidence and credibility.

2. **Playful** - Fun, friendly, approachable, maybe quirky. Think "Slack", "Discord", "Giphy". Names with personality that feel welcoming.

3. **Techy** - Cutting-edge, startup-y, Silicon Valley feel. Think "Vercel", "Supabase", "Deno", "Bun". Futuristic neologisms.

4. **Minimal** - Simple, elegant, understated. Think "Arc", "Craft", "Bear", "Loom". Short, memorable, less is more.

Guidelines:
- Generate 12-15 names per category (48-60 total)
- Prioritize .com domains (60%% of suggestions)
- Use invented/brandable words that are likely to be available
- Keep names short (ideally 4-10 characters before TLD)
- Avoid hyphens and numbers
- Be creative - don't use literal words from the description
- Each category should have a distinct feel

Return ONLY valid JSON in this exact format:
{
  "Professional": ["domain1.com", "domain2.io"],
  "Playful": ["domain3.com", "domain4.co"],
  "Techy": ["domain5.com", "domain6.dev"],
  "Minimal": ["domain7.com", "domain8.io"]
}`, description, strings.Join(tlds, ", "), exclusionNote)
}

func parseCategorizedDomains(text string) map[string][]string {
	// Find JSON object in response
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		return nil
	}

	jsonStr := text[start : end+1]

	var result map[string][]string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	// Clean up domains
	cleaned := make(map[string][]string)
	for cat, domains := range result {
		var cleanList []string
		for _, d := range domains {
			d = strings.TrimSpace(strings.ToLower(d))
			if d != "" && strings.Contains(d, ".") {
				cleanList = append(cleanList, d)
			}
		}
		if len(cleanList) > 0 {
			cleaned[cat] = cleanList
		}
	}

	return cleaned
}
