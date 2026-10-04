package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Attributes are the five detected values (REQ-003); "unknown" when unclear.
type Attributes struct {
	Colour   string `json:"colour"`
	Pattern  string `json:"pattern"`
	Sleeve   string `json:"sleeve"`
	Neckline string `json:"neckline"`
	Fit      string `json:"fit"`
}

// ListingText is a generated listing (REQ-004 to REQ-006).
type ListingText struct {
	Title       string    `json:"title"`
	Bullets     [5]string `json:"bullets"`
	Description string    `json:"description"`
}

// Product is what generation knows about a product.
type Product struct {
	SKU       string
	Brand     string
	Category  string
	VoiceNote string
}

// ChannelBrief is what generation must respect for one channel.
type ChannelBrief struct {
	Name           string
	TitleMaxLength int
	BannedWords    []string
}

// Template ids recorded on every call (AC-US-00-003-4: the prompt is traceable).
const (
	TemplateDetect     = "detect-v1"
	TemplateGenerate   = "generate-v1"
	TemplateRegenerate = "regenerate-v1"
)

// DetectPrompt asks for the five attributes of the garment in the photo.
func DetectPrompt(p Product) string {
	return fmt.Sprintf(`You are cataloguing an apparel product from its photo.
Product: SKU %s, category %s, brand %s.
Look only at the garment in the photo. Answer with JSON only, no prose:
{"colour": "...", "pattern": "...", "sleeve": "...", "neckline": "...", "fit": "..."}
Use short lowercase values such as "navy", "solid", "three-quarter sleeve", "round neck", "regular".
If a value cannot be seen in the photo, use "unknown". Never guess.`, p.SKU, p.Category, p.Brand)
}

// GeneratePrompt asks for one channel's listing in the brand's voice.
func GeneratePrompt(p Product, a Attributes, c ChannelBrief) string {
	voice := strings.TrimSpace(p.VoiceNote)
	if voice == "" {
		voice = "plain and neutral (the seller confirmed a neutral voice)"
	}
	return fmt.Sprintf(`Write a product listing for the %s channel.
Product: SKU %s, %s by %s.
Attributes: colour %s, pattern %s, sleeve %s, neckline %s, fit %s.
Brand voice: %s
Rules: the title is at most %d characters; never use these words or phrases: %s.
Describe only what the attributes say; invent no materials, sizes or claims.
Answer with JSON only:
{"title": "...", "bullets": ["...", "...", "...", "...", "..."], "description": "..."}
Exactly 5 bullets, each one short sentence.`,
		c.Name, p.SKU, p.Category, p.Brand, a.Colour, a.Pattern, a.Sleeve, a.Neckline, a.Fit,
		voice, c.TitleMaxLength, strings.Join(c.BannedWords, ", "))
}

// RegeneratePrompt asks for one field again, following the reviewer's instruction.
func RegeneratePrompt(p Product, a Attributes, c ChannelBrief, field, current, instruction string) string {
	limit := ""
	if field == "title" {
		limit = fmt.Sprintf(" It must be at most %d characters.", c.TitleMaxLength)
	}
	return fmt.Sprintf(`Rewrite the %s of a %s listing for SKU %s (%s by %s).
Attributes: colour %s, pattern %s, sleeve %s, neckline %s, fit %s.
Brand voice: %s
Current %s: %s
Reviewer's instruction: %s
Never use these words or phrases: %s.%s
Answer with JSON only: {"text": "..."}`,
		strings.ReplaceAll(field, "_", " "), c.Name, p.SKU, p.Category, p.Brand,
		a.Colour, a.Pattern, a.Sleeve, a.Neckline, a.Fit, orNeutral(p.VoiceNote),
		strings.ReplaceAll(field, "_", " "), current, instruction, strings.Join(c.BannedWords, ", "), limit)
}

func orNeutral(v string) string {
	if strings.TrimSpace(v) == "" {
		return "plain and neutral"
	}
	return v
}

var jsonObject = regexp.MustCompile(`(?s)\{.*\}`)

// ParseAttributes reads a detection answer. Missing values become "unknown".
func ParseAttributes(text string) (Attributes, error) {
	var a Attributes
	if err := json.Unmarshal([]byte(jsonObject.FindString(text)), &a); err != nil {
		return Attributes{}, fmt.Errorf("the model's answer was not the attribute JSON: %w", err)
	}
	for _, v := range []*string{&a.Colour, &a.Pattern, &a.Sleeve, &a.Neckline, &a.Fit} {
		*v = strings.ToLower(strings.TrimSpace(*v))
		if *v == "" {
			*v = "unknown"
		}
		if len(*v) > 100 {
			*v = (*v)[:100]
		}
	}
	return a, nil
}

// ParseListing reads a generation answer and refuses one without exactly 5 bullets.
func ParseListing(text string) (ListingText, error) {
	var raw struct {
		Title       string   `json:"title"`
		Bullets     []string `json:"bullets"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal([]byte(jsonObject.FindString(text)), &raw); err != nil {
		return ListingText{}, fmt.Errorf("the model's answer was not the listing JSON: %w", err)
	}
	if len(raw.Bullets) != 5 {
		return ListingText{}, fmt.Errorf("the model returned %d bullets, not 5", len(raw.Bullets))
	}
	l := ListingText{Title: clip(raw.Title, 500), Description: clip(raw.Description, 10000)}
	for i, b := range raw.Bullets {
		l.Bullets[i] = clip(b, 1000)
	}
	if l.Title == "" || l.Description == "" {
		return ListingText{}, fmt.Errorf("the model left the title or description empty")
	}
	return l, nil
}

// ParseField reads a regeneration answer.
func ParseField(text string, maxLen int) (string, error) {
	var raw struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(jsonObject.FindString(text)), &raw); err != nil {
		return "", fmt.Errorf("the model's answer was not the field JSON: %w", err)
	}
	t := clip(raw.Text, maxLen)
	if t == "" {
		return "", fmt.Errorf("the model returned empty text")
	}
	return t, nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
