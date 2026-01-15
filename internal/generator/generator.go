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

type Options struct {
	Vibe      string // professional, playful, techy, minimal
	NameStyle string // brandable, descriptive, compound
	Length    string // short, medium, any
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

func (g *Generator) Generate(ctx context.Context, description string, tlds []string, opts Options) ([]string, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := buildPrompt(description, tlds, opts)

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

func buildPrompt(description string, tlds []string, opts Options) string {
	// Build vibe guidance
	vibeGuide := ""
	switch opts.Vibe {
	case "professional":
		vibeGuide = `- VIBE: Professional and trustworthy. Think established companies, corporate feel, credible and authoritative. Names like "Stripe", "Notion", "Linear", "Figma". Clean, serious, instills confidence.`
	case "playful":
		vibeGuide = `- VIBE: Playful and fun. Think friendly, approachable, maybe a bit quirky or whimsical. Names like "Slack", "Discord", "Giphy", "Wobble". Can be cute, surprising, or have personality.`
	case "techy":
		vibeGuide = `- VIBE: Techy and startup-y. Think Silicon Valley, cutting-edge, modern tech. Names like "Vercel", "Supabase", "Deno", "Bun". Can sound technical, futuristic, or use tech-inspired neologisms.`
	case "minimal":
		vibeGuide = `- VIBE: Minimal and clean. Think simple, elegant, understated. Names like "Arc", "Linear", "Craft", "Bear". Short, memorable single words or very tight combinations. Less is more.`
	default:
		vibeGuide = `- VIBE: Balanced and brandable. Professional but modern.`
	}

	// Build name style guidance
	styleGuide := ""
	switch opts.NameStyle {
	case "brandable":
		styleGuide = `- STYLE: Invented/brandable names. Create completely new words that sound good and are memorable. Think "Spotify", "Klarna", "Twilio", "Zapier". Made-up words, creative letter combinations, words that feel like they could be real but aren't. Highly likely to have available .com domains.`
	case "descriptive":
		styleGuide = `- STYLE: Descriptive names that hint at what the product does. Think "Dropbox", "Salesforce", "Mailchimp", "Grammarly". The name gives a clue about the function or benefit. Can use metaphors or word plays.`
	case "compound":
		styleGuide = `- STYLE: Compound words - two real words merged together. Think "Facebook", "YouTube", "WordPress", "Snapchat". Combine relevant concepts, actions, or metaphors into memorable mashups.`
	default:
		styleGuide = `- STYLE: Mix of brandable invented words and clever compounds.`
	}

	// Build length guidance
	lengthGuide := ""
	switch opts.Length {
	case "short":
		lengthGuide = `- LENGTH: Keep it SHORT. Maximum 6 characters before the TLD. Single syllable or very tight two-syllable names. Think "Arc", "Dub", "Loom", "Zoom", "Miro".`
	case "medium":
		lengthGuide = `- LENGTH: Medium length, 7-10 characters before the TLD. Sweet spot for memorability and brandability. Think "Notion", "Figma", "Stripe", "Canva".`
	case "any":
		lengthGuide = `- LENGTH: Any length that sounds good and is memorable. Prioritize the best-sounding names regardless of length.`
	default:
		lengthGuide = `- LENGTH: Aim for 6-10 characters before the TLD.`
	}

	return fmt.Sprintf(`Generate 50 creative domain name suggestions based on this project description:

"%s"

TLDs to use: %s

Style Guidelines:
%s
%s
%s

Technical Guidelines:
- IMPORTANT: Generate at least 60%% of suggestions as .com domains since they're most desirable
- Use unusual, invented, or uncommon words that are more likely to be available
- Make names memorable, brandable, and easy to spell
- Avoid hyphens and numbers
- Be creative - capture the essence, don't just use literal words from the description
- Think like a startup founder looking for an available domain - get creative with spelling and word combinations

Output format: Return ONLY a JSON array of domain names, nothing else. Example:
["brandname.com", "coolstartup.io", "myproject.ai"]`, description, strings.Join(tlds, ", "), vibeGuide, styleGuide, lengthGuide)
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
