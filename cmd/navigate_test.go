package cmd

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"example.com":             "https://example.com",
		"example.com/a?b=1":       "https://example.com/a?b=1",
		"localhost:3000":          "http://localhost:3000",
		"app.localhost/x":         "http://app.localhost/x",
		"127.0.0.1:8080/x":        "http://127.0.0.1:8080/x",
		"https://example.com":     "https://example.com",
		"http://example.com":      "http://example.com",
		"file:///tmp/a.html":      "file:///tmp/a.html",
		"chrome://version":        "chrome://version",
		"about:blank":             "about:blank",
		"data:text/html,<p>x</p>": "data:text/html,<p>x</p>",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}
