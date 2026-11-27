package cmd

import "testing"

func TestSelectorKind(t *testing.T) {
	cases := map[string]string{
		"#submit":             "css",
		"div.card":            "css",
		"text=Sign in":        "text",
		"//tr[3]//a":          "xpath",
		"./a":                 "xpath",
		"(//a)[2]":            "xpath",
		".toolbar >> text=Go": "chain",
		"my-player >>> .btn":  "chain",
		"#c text=Speichern":   "chain",
		"  text=padded  ":     "text",
	}
	for sel, want := range cases {
		if got := SelectorKind(sel); got != want {
			t.Errorf("SelectorKind(%q) = %q, want %q", sel, got, want)
		}
	}
}

func TestTextAlternative(t *testing.T) {
	cases := []struct {
		sel, kind, want string
	}{
		{"Feed suchen", "css", "text=Feed suchen"},
		{"Speichern", "css", "text=Speichern"},
		{"über", "css", "text=über"},
		{"button", "css", ""},
		{"my-element", "css", ""},
		{"#id", "css", ""},
		{"div > span", "css", ""},
		{"[name=q]", "css", ""},
		{"Feed suchen", "text", ""},
		{"Feed suchen", "xpath", ""},
	}
	for _, c := range cases {
		if got := textAlternative(c.sel, c.kind); got != c.want {
			t.Errorf("textAlternative(%q, %q) = %q, want %q", c.sel, c.kind, got, c.want)
		}
	}
}
