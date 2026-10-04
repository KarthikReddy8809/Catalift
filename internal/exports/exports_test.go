package exports

import "testing"

func TestEscapeCell(t *testing.T) {
	cases := []struct{ in, want string }{
		{"=HYPERLINK(1)", "'=HYPERLINK(1)"},
		{"+1", "'+1"},
		{"-1", "'-1"},
		{"@SUM", "'@SUM"},
		{"Navy kurta", "Navy kurta"},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := EscapeCell(c.in)

			if got != c.want {
				t.Fatalf("EscapeCell(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
