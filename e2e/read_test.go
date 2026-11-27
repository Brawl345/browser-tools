package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type elementOutput struct {
	Count    int              `json:"count"`
	Elements []map[string]any `json:"elements"`
	Note     string           `json:"note"`
}

func element(t *testing.T, args ...string) elementOutput {
	t.Helper()
	r := ok(t, append([]string{"element"}, args...)...)
	var out elementOutput
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("element output is not JSON: %v\n%s", err, r.stdout)
	}
	return out
}

func TestElementSingleMatch(t *testing.T) {
	open(t, "/list.html")
	out := element(t, "h1")
	if out.Count != 1 || len(out.Elements) != 1 {
		t.Fatalf("count %d, %d elements", out.Count, len(out.Elements))
	}
	el := out.Elements[0]
	if el["tag"] != "h1" || el["text"] != "Products" || el["visible"] != true {
		t.Errorf("h1 entry = %v", el)
	}
	for _, key := range []string{"box", "style", "attributes", "html"} {
		if _, has := el[key]; !has {
			t.Errorf("a single match lacks %q", key)
		}
	}
}

func TestElementManyMatches(t *testing.T) {
	open(t, "/list.html")
	out := element(t, "li")
	if out.Count != 12 || len(out.Elements) != 10 {
		t.Errorf("count %d, %d elements, want 12 and 10", out.Count, len(out.Elements))
	}
	if !strings.Contains(out.Note, "showing 10 of 12") {
		t.Errorf("note = %q", out.Note)
	}
	if _, has := out.Elements[0]["attributes"]; has {
		t.Error("the short form carries attributes")
	}

	limited := element(t, "li", "--limit", "3")
	if len(limited.Elements) != 3 || limited.Elements[2]["text"] != "Item 3" {
		t.Errorf("--limit 3 gave %v", limited.Elements)
	}
}

func TestElementAttr(t *testing.T) {
	open(t, "/list.html")
	r := ok(t, "element", "li", "--attr", "data-id")
	got := lines(r.stdout)
	if len(got) != 12 || got[0] != "1" || got[11] != "12" {
		t.Errorf("data-id lines = %q", got)
	}

	r = ok(t, "element", "li", "--attr", "text", "--limit", "4")
	if got := lines(r.stdout); len(got) != 4 || got[3] != "Item 4" {
		t.Errorf("--limit 4 lines = %q", got)
	}
	contains(t, r, "printed 4 of 12 matches")

	contains(t, ok(t, "element", "#more", "--attr", "href"), "/blank.html")
	contains(t, ok(t, "element", "#more", "--attr", "html"), `<a id="more"`)
}

func TestElementAttrMissing(t *testing.T) {
	open(t, "/list.html")
	r := fails(t, "element", "h1", "--attr", "data-nothing")
	contains(t, r, `carries "data-nothing"`)
}

func TestElementLimitRejectsNegative(t *testing.T) {
	open(t, "/list.html")
	contains(t, fails(t, "element", "li", "--limit", "-1"), "--limit must be at least 1")
}

func TestHTMLFilter(t *testing.T) {
	open(t, "/list.html")
	r := ok(t, "html", "--filter", `id="more"`, "--lines", "0")
	if got := lines(r.stdout); len(got) != 1 || !strings.HasPrefix(got[0], `<a id="more"`) {
		t.Errorf("--lines 0 printed %q", got)
	}
	if got := lines(ok(t, "html", "--filter", `id="more"`, "--lines", "1").stdout); len(got) != 3 {
		t.Errorf("--lines 1 printed %d lines, want the match and one line around it", len(got))
	}
	contains(t, fails(t, "html", "--filter", "nothing-like-this"), "no matches found")
	contains(t, ok(t, "html"), "<title>Products</title>", "</html>")
}

func TestEvaluateJS(t *testing.T) {
	open(t, "/list.html")
	cases := []struct {
		name, js, want string
	}{
		{"expression", "1 + 1", "2"},
		{"string unquoted", "document.title", "Products"},
		{"top-level return", "const h = document.querySelector('h1'); return h.textContent", "Products"},
		{"early return", "if (true) { return 'early' } 'late'", "early"},
		{"top-level await", "await new Promise(r => setTimeout(r, 20)); 'waited'", "waited"},
		{"await with return", "await 1; return 'x'", "x"},
		{"async IIFE", "(async () => { await 1; return 5 })()", "5"},
		{"promise result", "fetch(location.href).then(r => r.status)", "200"},
		{"object as JSON", "return {a: 1, b: [1, 2]}", `{"a":1,"b":[1,2]}`},
		{"array as JSON", "[...document.querySelectorAll('li')].slice(0, 2).map(e => e.dataset.id)", `["1","2"]`},
		{"stringified stays JSON", "JSON.stringify({a: 1})", `{"a":1}`},
		{"DOM node", "document.querySelector('h1')", "h1.title"},
		{"map", "new Map([[1, 2]])", "Map(1)"},
		{"null", "null", "null"},
		{"infinity", "1/0", "Infinity"},
		{"undefined", "undefined", ""},
		{"let again", "let q = 1; q", "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.TrimSpace(ok(t, "evaluate-js", c.js).stdout)
			if got != c.want {
				t.Errorf("evaluate-js %q = %q, want %q", c.js, got, c.want)
			}
		})
	}
}

func TestEvaluateJSErrors(t *testing.T) {
	open(t, "/list.html")
	contains(t, fails(t, "evaluate-js", "throw new Error('boom')"), "Error: boom")
	contains(t, fails(t, "evaluate-js", "Promise.reject(new Error('nope'))"), "Error: nope")
	contains(t, fails(t, "evaluate-js", "syntax error here("), "SyntaxError")
}

func TestEvaluateJSFile(t *testing.T) {
	open(t, "/list.html")
	path := filepath.Join(t.TempDir(), "count.js")
	if err := os.WriteFile(path, []byte("return document.querySelectorAll('li').length"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(ok(t, "evaluate-js", path).stdout); got != "12" {
		t.Errorf("script file = %q", got)
	}
}

func TestEvaluateJSHints(t *testing.T) {
	open(t, "/list.html")
	contains(t, ok(t, "evaluate-js", "document.querySelector('h1').textContent"), "hint: `browser-tools element")
}

func TestConsole(t *testing.T) {
	open(t, "/console.html")
	r := ok(t, "console")
	contains(t, r, "[LOG] boot 1", "[LOG] boot 5", "[WARNING] careful", "[ERROR] bad thing")

	r = ok(t, "console", "--limit", "2")
	contains(t, r, "5 earlier messages skipped", "[WARNING] careful", "[ERROR] bad thing")
	lacks(t, r, "boot 5")

	r = ok(t, "console", "--errors-only")
	contains(t, r, "bad thing")
	lacks(t, r, "boot", "careful")
}

func TestConsoleEmpty(t *testing.T) {
	open(t, "/blank.html")
	contains(t, ok(t, "console"), "No console messages.")
}
