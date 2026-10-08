package ai

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestParseListingNeedsExactlyFiveBullets(t *testing.T) {
	_, err := ParseListing(`{"title":"T","bullets":["a","b","c","d"],"description":"D"}`)

	if err == nil || !strings.Contains(err.Error(), "4 bullets") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseListingReadsJSONInsideProse(t *testing.T) {
	l, err := ParseListing("Here it is:\n" + `{"title":"Navy Kurta","bullets":["a","b","c","d","e"],"description":"D"}`)

	if err != nil || l.Title != "Navy Kurta" || l.Bullets[4] != "e" {
		t.Fatalf("l=%+v err=%v", l, err)
	}
}

func TestParseAttributesFillsUnknown(t *testing.T) {
	a, err := ParseAttributes(`{"colour":"Navy","pattern":"","fit":"regular"}`)

	if err != nil || a.Colour != "navy" || a.Pattern != "unknown" || a.Sleeve != "unknown" {
		t.Fatalf("a=%+v err=%v", a, err)
	}
}

func pngOf(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img) // encoding an in-memory image cannot fail
	return b.Bytes()
}

func TestLocalDetectReadsThePhotoColour(t *testing.T) {
	resp, err := Local{}.Complete(context.Background(), Request{
		Purpose: PurposeDetect, Prompt: DetectPrompt(Product{SKU: "KU-1", Brand: "B", Category: "kurta"}),
		Image: pngOf(color.RGBA{R: 22, G: 32, B: 85, A: 255}), ImageType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}

	a, err := ParseAttributes(resp.Text)

	if err != nil || a.Colour != "navy" || resp.CostMicroUSD != 0 {
		t.Fatalf("a=%+v cost=%d err=%v", a, resp.CostMicroUSD, err)
	}
}

func TestLocalGenerateKeepsTheTitleLimitAndFiveBullets(t *testing.T) {
	prompt := GeneratePrompt(
		Product{SKU: "KU-1", Brand: "Indigo Loom", Category: "kurta"},
		Attributes{Colour: "navy", Pattern: "solid", Sleeve: "three-quarter sleeve", Neckline: "round neck", Fit: "regular"},
		ChannelBrief{Name: "Amazon-style", TitleMaxLength: 30, BannedWords: []string{"sale"}},
	)
	resp, err := Local{}.Complete(context.Background(), Request{Purpose: PurposeGenerate, Prompt: prompt})
	if err != nil {
		t.Fatal(err)
	}

	l, err := ParseListing(resp.Text)

	if err != nil || len([]rune(l.Title)) > 30 || l.Bullets[4] == "" {
		t.Fatalf("l=%+v err=%v", l, err)
	}
}

func TestLocalEnrichAnswersEveryChannelInOneCall(t *testing.T) {
	prompt := EnrichPrompt(
		Product{SKU: "KU-1", Brand: "Indigo Loom", Category: "kurta"},
		[]ChannelBrief{
			{ID: "amazon_style", Name: "Amazon-style", TitleMaxLength: 40, BannedWords: []string{"sale"}},
			{ID: "own_website", Name: "Own website", TitleMaxLength: 120},
		},
	)
	resp, err := Local{}.Complete(context.Background(), Request{
		Purpose: PurposeDetect, TemplateID: TemplateEnrich, Prompt: prompt,
		Image: pngOf(color.RGBA{R: 22, G: 32, B: 85, A: 255}), ImageType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}

	e, err := ParseEnrichment(resp.Text, []string{"amazon_style", "own_website"})

	if err != nil || e.Attributes.Colour != "navy" || len(e.Listings) != 2 ||
		len([]rune(e.Listings["amazon_style"].Title)) > 40 || e.Confidence <= 0 || e.Confidence > 1 {
		t.Fatalf("e=%+v err=%v", e, err)
	}
}

func TestParseEnrichmentRefusesAMissingChannel(t *testing.T) {
	text := `{"attributes":{"colour":"navy"},"confidence":0.9,"listings":{"a":{"title":"T","bullets":["1","2","3","4","5"],"description":"D"}}}`

	_, err := ParseEnrichment(text, []string{"a", "b"})

	if err == nil || !strings.Contains(err.Error(), "listing for b") {
		t.Fatalf("err = %v", err)
	}
}

func TestPromptsCarryTheBrandsWordsToAvoid(t *testing.T) {
	chans := []ChannelBrief{{ID: "own_website", Name: "Own website", TitleMaxLength: 120}}

	with := EnrichPrompt(Product{SKU: "KU-1", Brand: "B", Category: "kurta", AvoidWords: []string{"cheap", "best ever"}}, chans)
	without := EnrichPrompt(Product{SKU: "KU-1", Brand: "B", Category: "kurta"}, chans)

	if !strings.Contains(with, "The brand never uses these words or phrases: cheap, best ever.") {
		t.Fatalf("words to avoid missing from the prompt:\n%s", with)
	}
	if strings.Contains(without, "never uses these words") {
		t.Fatalf("a brand with no words to avoid got the line:\n%s", without)
	}
}
