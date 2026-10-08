package channels

import (
	"strings"
	"testing"
)

func testChannel() Channel {
	return Channel{
		ID:                 "amazon_style",
		Name:               "Amazon-style",
		TitleMaxLength:     20,
		RequiredAttributes: []string{"colour", "fit"},
		BannedWords:        []string{"easy care", "sale"},
	}
}

func goodListing() Listing {
	return Listing{
		Title:       "Navy Cotton Kurta",
		Bullets:     [5]string{"Soft cotton", "Regular fit", "Three-quarter sleeves", "Round neck", "Machine wash cold"},
		Description: "An everyday navy kurta.",
		Attributes:  map[string]string{"colour": "navy", "fit": "regular"},
	}
}

func rules(fs []Failure) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Rule+":"+f.Field)
	}
	return out
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Listing)
		want   []string
	}{
		{name: "a clean listing passes", mutate: func(*Listing) {}, want: []string{}},
		{
			name:   "title over the limit names its length",
			mutate: func(l *Listing) { l.Title = strings.Repeat("a", 21) },
			want:   []string{"title_max_length:title"},
		},
		{
			name:   "title at the limit passes",
			mutate: func(l *Listing) { l.Title = strings.Repeat("a", 20) },
			want:   []string{},
		},
		{
			name:   "banned phrase in a bullet, ignoring case",
			mutate: func(l *Listing) { l.Bullets[4] = "Easy Care fabric" },
			want:   []string{"banned_word:bullet_5"},
		},
		{
			name:   "banned word matches whole words only",
			mutate: func(l *Listing) { l.Description = "Wholesale-ready salesman cut" },
			want:   []string{},
		},
		{
			name:   "missing required attribute",
			mutate: func(l *Listing) { l.Attributes["fit"] = "" },
			want:   []string{"required_attribute:fit"},
		},
		{
			name:   "unknown counts as missing (AC-US-00-002-3)",
			mutate: func(l *Listing) { l.Attributes["colour"] = "unknown" },
			want:   []string{"required_attribute:colour"},
		},
		{
			name: "a brand's word to avoid in the description",
			mutate: func(l *Listing) {
				l.AvoidWords = []string{"cheap"}
				l.Description = "A Cheap, everyday navy kurta."
			},
			want: []string{"brand_avoid_word:description"},
		},
		{
			name: "a brand's words to avoid match whole words only",
			mutate: func(l *Listing) {
				l.AvoidWords = []string{"cheap"}
				l.Description = "Cheaply priced? No: cheapest is not a word we use."
			},
			want: []string{},
		},
		{
			name:   "empty bullet",
			mutate: func(l *Listing) { l.Bullets[2] = "  " },
			want:   []string{"empty_field:bullet_3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := goodListing()
			tc.mutate(&l)

			got := rules(Validate(testChannel(), l))

			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateMessageNamesLimitAndLength(t *testing.T) {
	l := goodListing()
	l.Title = strings.Repeat("a", 25)

	fs := Validate(testChannel(), l)

	if len(fs) != 1 || fs[0].Message != "Title is 25 characters; the Amazon-style limit is 20." {
		t.Fatalf("message: %+v", fs)
	}
}

func TestValidateIsDeterministic(t *testing.T) {
	l := goodListing()
	l.Title = strings.Repeat("a", 25)
	l.Bullets[0] = "sale today"

	a, b := Validate(testChannel(), l), Validate(testChannel(), l)

	if strings.Join(rules(a), ",") != strings.Join(rules(b), ",") {
		t.Fatalf("two runs differ: %v %v", rules(a), rules(b))
	}
}
