package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/jpeg" // decoders for the detection copy
	_ "image/png"
	"math"
	"regexp"
	"strings"

	_ "golang.org/x/image/webp"
)

// Local is the stand-in provider used when no OpenRouter key is set: it
// costs nothing and answers deterministically, so the whole flow can be
// clicked through without spending budget. Detection reads the photo's
// dominant colour for real; the other attributes are picked from the SKU.
type Local struct{}

// Name identifies the provider in logs and on the ledger's model column.
func (Local) Name() string { return "local-standin" }

var (
	skuRe   = regexp.MustCompile(`SKU (\S+)`)
	fieldRe = regexp.MustCompile(`Rewrite the (.+?) of a`)
	attrsRe = regexp.MustCompile(`colour (.+?), pattern (.+?), sleeve (.+?), neckline (.+?), fit (.+?)\.`)
	brandRe = regexp.MustCompile(`, (.+?) by (.+?)\.`)
	chanRe  = regexp.MustCompile(`for the (.+?) channel`)
	limitRe = regexp.MustCompile(`at most (\d+) characters`)
	instrRe = regexp.MustCompile(`Reviewer's instruction: (.+)`)
	catRe   = regexp.MustCompile(`category (.+?), brand (.+?)\.`)
	enrChRe = regexp.MustCompile(`- Channel (\S+) \((.+?)\): the title is at most (\d+) characters`)
)

// Complete answers in the same JSON shapes a real model is asked for.
func (Local) Complete(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, fmt.Errorf("local stand-in: %w", err)
	}
	var out any
	switch req.Purpose {
	case PurposeDetect, PurposeEvalDetect:
		if req.TemplateID == TemplateEnrich {
			out = localEnrich(req)
		} else {
			out = localDetect(req)
		}
	case PurposeGenerate:
		out = localGenerate(req.Prompt)
	case PurposeRegenerate:
		out = map[string]string{"text": localRegenerate(req.Prompt)}
	default:
		return Response{}, fmt.Errorf("local provider: unknown purpose %q", req.Purpose)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return Response{}, fmt.Errorf("local provider: %w", err)
	}
	return Response{
		Text: string(b), InputTokens: len(req.Prompt) / 4, OutputTokens: len(b) / 4,
		CostMicroUSD: 0, Model: "local-standin",
	}, nil
}

func pick(seed string, options []string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))                        // fnv never fails
	return options[int(h.Sum32()%uint32(len(options)))] //nolint:gosec // US-00-002: len(options) is a small constant.
}

func localDetect(req Request) Attributes {
	sku := ""
	if m := skuRe.FindStringSubmatch(req.Prompt); m != nil {
		sku = strings.TrimSuffix(m[1], ",")
	}
	colour := "unknown"
	if img, _, err := image.Decode(bytes.NewReader(req.Image)); err == nil {
		colour = dominantColour(img)
	}
	return Attributes{
		Colour:   colour,
		Pattern:  pick(sku+"p", []string{"solid", "striped", "checked", "floral", "block print"}),
		Sleeve:   pick(sku+"s", []string{"short sleeve", "three-quarter sleeve", "full sleeve", "sleeveless"}),
		Neckline: pick(sku+"n", []string{"round neck", "v-neck", "mandarin collar", "boat neck"}),
		Fit:      pick(sku+"f", []string{"regular", "straight", "a-line", "relaxed"}),
	}
}

var palette = []struct {
	name    string
	r, g, b float64
}{
	{"black", 20, 20, 20}, {"white", 240, 240, 240}, {"grey", 128, 128, 128},
	{"navy", 20, 30, 80}, {"blue", 40, 90, 200}, {"red", 200, 30, 40},
	{"maroon", 110, 20, 30}, {"green", 40, 150, 60}, {"olive", 110, 110, 40},
	{"yellow", 230, 200, 40}, {"orange", 235, 130, 30}, {"pink", 235, 140, 170},
	{"purple", 120, 50, 150}, {"brown", 110, 70, 40}, {"beige", 215, 195, 160}, {"rust", 170, 80, 40},
}

// dominantColour averages the centre of the image (where the garment is)
// and names the nearest palette colour.
func dominantColour(img image.Image) string {
	b := img.Bounds()
	x0, x1 := b.Min.X+b.Dx()/4, b.Max.X-b.Dx()/4
	y0, y1 := b.Min.Y+b.Dy()/4, b.Max.Y-b.Dy()/4
	var r, g, bl, n float64
	for y := y0; y < y1; y += 4 {
		for x := x0; x < x1; x += 4 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r += float64(cr >> 8)
			g += float64(cg >> 8)
			bl += float64(cb >> 8)
			n++
		}
	}
	if n == 0 {
		return "unknown"
	}
	r, g, bl = r/n, g/n, bl/n
	best, bestD := "unknown", math.MaxFloat64
	for _, p := range palette {
		d := (r-p.r)*(r-p.r) + (g-p.g)*(g-p.g) + (bl-p.b)*(bl-p.b)
		if d < bestD {
			best, bestD = p.name, d
		}
	}
	return best
}

func localGenerate(prompt string) map[string]any {
	a := Attributes{Colour: "unknown", Pattern: "unknown", Sleeve: "unknown", Neckline: "unknown", Fit: "unknown"}
	if m := attrsRe.FindStringSubmatch(prompt); m != nil {
		a = Attributes{Colour: m[1], Pattern: m[2], Sleeve: m[3], Neckline: m[4], Fit: m[5]}
	}
	category, brand := "garment", "the brand"
	if m := brandRe.FindStringSubmatch(prompt); m != nil {
		category, brand = m[1], m[2]
	}
	channel := ""
	if m := chanRe.FindStringSubmatch(prompt); m != nil {
		channel = m[1]
	}
	limit := 0
	if m := limitRe.FindStringSubmatch(prompt); m != nil {
		_, _ = fmt.Sscanf(m[1], "%d", &limit) // the regexp guarantees digits
	}
	return localListing(a, category, brand, channel, limit)
}

// localEnrich answers EnrichPrompt: the photo's colour, picked attributes, a
// confidence, and a listing per channel named in the prompt.
func localEnrich(req Request) map[string]any {
	a := localDetect(req)
	category, brand := "garment", "the brand"
	if m := catRe.FindStringSubmatch(req.Prompt); m != nil {
		category, brand = m[1], m[2]
	}
	sku := ""
	if m := skuRe.FindStringSubmatch(req.Prompt); m != nil {
		sku = strings.TrimSuffix(m[1], ",")
	}
	// Mostly sure; some products low, so the reviewer's triage filter has work.
	confidence := map[string]float64{"a": 0.94, "b": 0.88, "c": 0.81, "d": 0.42}[pick(sku+"c", []string{"a", "a", "b", "c", "d"})]
	if a.Colour == "unknown" {
		confidence = 0.3
	}
	listings := map[string]any{}
	for _, m := range enrChRe.FindAllStringSubmatch(req.Prompt, -1) {
		var limit int
		_, _ = fmt.Sscanf(m[3], "%d", &limit) // the regexp guarantees digits
		listings[m[1]] = localListing(a, category, brand, m[2], limit)
	}
	return map[string]any{"attributes": a, "confidence": confidence, "listings": listings}
}

func localListing(a Attributes, category, brand, channel string, limit int) map[string]any {
	colour, pattern, sleeve, neck, fit := a.Colour, a.Pattern, a.Sleeve, a.Neckline, a.Fit
	title := fmt.Sprintf("%s %s %s %s, %s fit, %s", titleCase(brand), titleCase(colour), titleCase(pattern), titleCase(category), titleCase(fit), titleCase(sleeve))
	if channel == "Own website" {
		title = fmt.Sprintf("%s %s %s", titleCase(colour), titleCase(pattern), titleCase(category))
	}
	if limit > 0 && len([]rune(title)) > limit {
		title = string([]rune(title)[:limit])
	}
	return map[string]any{
		"title": title,
		"bullets": []string{
			fmt.Sprintf("A %s %s %s in a %s finish.", colour, pattern, category, pattern),
			fmt.Sprintf("%s fit that sits comfortably through the day.", titleCase(fit)),
			fmt.Sprintf("%s for easy movement.", titleCase(sleeve)),
			fmt.Sprintf("Finished with a %s.", neck),
			fmt.Sprintf("Made by %s.", brand),
		},
		"description": fmt.Sprintf("A %s %s %s from %s with a %s and %s, cut in a %s fit.", colour, pattern, category, brand, neck, sleeve, fit),
	}
}

func localRegenerate(prompt string) string {
	field := "text"
	if m := fieldRe.FindStringSubmatch(prompt); m != nil {
		field = m[1]
	}
	instr := ""
	if m := instrRe.FindStringSubmatch(prompt); m != nil {
		instr = strings.TrimSpace(m[1])
	}
	a := []string{"", "", "", "", ""}
	if m := attrsRe.FindStringSubmatch(prompt); m != nil {
		a = m[1:6]
	}
	text := fmt.Sprintf("%s %s %s, %s fit", titleCase(a[0]), titleCase(a[1]), field, a[4])
	if field == "title" {
		text = fmt.Sprintf("%s %s Kurta, %s Fit", titleCase(a[0]), titleCase(a[1]), titleCase(a[4]))
	}
	if strings.Contains(strings.ToLower(instr), "shorter") {
		text = strings.SplitN(text, ",", 2)[0]
	}
	if m := limitRe.FindStringSubmatch(prompt); m != nil {
		var limit int
		_, _ = fmt.Sscanf(m[1], "%d", &limit) // the regexp guarantees digits
		if limit > 0 && len([]rune(text)) > limit {
			text = string([]rune(text)[:limit])
		}
	}
	return text
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
