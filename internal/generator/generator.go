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
		client: &http.Client{}, // No timeout - let context handle cancellation
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
		Model:     "claude-haiku-4-5-20251001",
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

// GenerateFromDomainIdea generates variations of a domain idea
func (g *Generator) GenerateFromDomainIdea(ctx context.Context, domainIdea string, tlds []string) (map[string][]string, error) {
	return g.GenerateFromDomainIdeaWithExclusions(ctx, domainIdea, tlds, nil)
}

// GenerateFromDomainIdeaWithExclusions generates variations of a domain idea, avoiding taken domains
func (g *Generator) GenerateFromDomainIdeaWithExclusions(ctx context.Context, domainIdea string, tlds []string, takenDomains []string) (map[string][]string, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := buildDomainExplorationPrompt(domainIdea, tlds, takenDomains)

	reqBody := claudeRequest{
		Model:     "claude-haiku-4-5-20251001",
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

func buildDomainExplorationPrompt(domainIdea string, tlds []string, takenDomains []string) string {
	exclusionNote := ""
	if len(takenDomains) > 0 {
		maxToShow := 30
		if len(takenDomains) > maxToShow {
			takenDomains = takenDomains[:maxToShow]
		}
		exclusionNote = fmt.Sprintf(`

IMPORTANT - These domains are TAKEN. Avoid these exact names:
%s

Generate DIFFERENT variations that aren't in this list.`, strings.Join(takenDomains, ", "))
	}

	return fmt.Sprintf(`The user has a domain idea: "%s"

Your job is to EXPLORE THE SHIT out of this domain idea and generate amazing variations. Think of yourself as a creative domain naming expert who's been given a seed idea to riff on.

TLDs to use: %s%s

First, analyze the domain idea:
- What's the core word/concept?
- What might this domain be for? (personal brand, business, project?)
- What makes it memorable or interesting?

Then generate variations in 4 categories:

1. **Professional** - Clean variations that maintain the professional vibe
   - Same word with different TLDs
   - The word + common professional suffixes (hq, pro, labs, studio, co, group)
   - Slight spelling variations that look intentional
   - First name / last name combinations if it appears to be a personal brand

2. **Playful** - Fun spins on the original
   - Add "get", "hey", "go", "try", "meet", "hello" prefixes
   - Add "app", "me", "now", "daily", "club" suffixes
   - Creative wordplay that keeps the essence
   - Alliterative or rhyming variations

3. **Creative** - Unexpected twists and invented words
   - Portmanteaus combining parts of the idea with other concepts
   - Abstract variations that evoke the same feeling
   - Phonetic respellings that look cool
   - Compound words that expand on the idea
   - Foreign language equivalents or inspired words

4. **Minimal** - Short, punchy alternatives
   - Abbreviations or initials if applicable
   - Single-syllable alternatives
   - The shortest possible versions that still work
   - Premium single-word domains in the same space

Guidelines:
- Generate 12-15 names per category (48-60 total)
- Mix TLDs naturally throughout
- The original domain (with various TLDs) should appear in the results
- Be creative but keep variations RELATED to the original idea
- No hyphens or numbers
- Think about what variations the user hasn't thought of yet

Return ONLY valid JSON in this exact format:
{
  "Professional": ["domain1.com", "domain2.io"],
  "Playful": ["domain3.com", "domain4.co"],
  "Creative": ["domain5.com", "domain6.dev"],
  "Minimal": ["domain7.com", "domain8.io"]
}`, domainIdea, strings.Join(tlds, ", "), exclusionNote)
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

	return fmt.Sprintf(`Generate domain name suggestions for this project, organized by style:

PROJECT: "%s"

TLDs to use: %s%s

First, identify 5-10 relevant keywords, concepts, and metaphors for this project. Think about:
- Industry-specific terminology
- Related activities, tools, or settings
- Emotions or outcomes the project evokes
- Geographic or cultural references if relevant

Then generate names in 4 categories:

1. **Professional** - Clean, credible, straightforward. Names that clearly communicate what the business does. Think: established brands, trustworthy institutions.

2. **Playful** - Fun, friendly, memorable. Names with personality that make people smile. Think: wordplay that actually relates to the business, friendly mascot-style names, approachable compounds.

3. **Creative** - Clever inventions that still connect to the business. Unexpected portmanteaus, surprising metaphors, or invented words that evoke the right feeling. Go beyond the obvious combinations - find the unexpected angle. Must still be relevant but shouldn't be the first thing anyone would think of.

4. **Minimal** - Short, clean, elegant. Single words or tight two-syllable compounds. Premium feel. Think: uncommon but real dictionary words, poetic terms, or unexpected single-word metaphors. Avoid obvious category words that everyone would think of.

Guidelines:
- Generate 12-15 names per category (48-60 total)
- Mix TLDs naturally - use .com where it fits, but don't force it
- **CRITICAL: Every name must clearly relate to the project.** A stranger should be able to guess the business from the domain name.
- **Never truncate or abbreviate words** - use complete words only. "cric", "crick", "btfy" are all bad. If a word is too long, use a different word entirely or a related synonym.
- Aim for 6-14 characters before the TLD - short enough to type, long enough to be complete words
- No hyphens or numbers
- Invented words are fine but must evoke the business (e.g., "Spotify" evokes music/audio)

Return ONLY valid JSON in this exact format:
{
  "Professional": ["domain1.com", "domain2.io"],
  "Playful": ["domain3.com", "domain4.co"],
  "Creative": ["domain5.com", "domain6.dev"],
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
