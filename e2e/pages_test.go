package e2e

import (
	"strings"
	"testing"
)

func TestNavigate(t *testing.T) {
	reset(t)
	r := ok(t, "navigate", pageURL("/list.html"))
	contains(t, r, "Navigated to", "/list.html")
	if got := ok(t, "evaluate-js", "document.title").stdout; strings.TrimSpace(got) != "Products" {
		t.Errorf("title = %q", got)
	}
}

func TestNavigateFollowsRedirect(t *testing.T) {
	reset(t)
	ok(t, "navigate", pageURL("/redirect"))
	if got := strings.TrimSpace(ok(t, "evaluate-js", "location.pathname").stdout); got != "/list.html" {
		t.Errorf("pathname after redirect = %q", got)
	}
}

func TestNavigateWaitsForSlowPage(t *testing.T) {
	reset(t)
	ok(t, "navigate", pageURL("/slow"))
	r := ok(t, "element", "#slow", "--attr", "text")
	if strings.TrimSpace(r.stdout) != "done" {
		t.Errorf("#slow = %q", r.stdout)
	}
}

func TestNavigateOpaqueURLs(t *testing.T) {
	reset(t)
	ok(t, "navigate", "data:text/html,<p id=d>data page</p>")
	contains(t, ok(t, "element", "#d", "--attr", "text"), "data page")
	ok(t, "navigate", "about:blank")
	contains(t, ok(t, "evaluate-js", "location.href"), "about:blank")
}

func TestReload(t *testing.T) {
	open(t, "/list.html")
	ok(t, "evaluate-js", "window.marker = 1")
	ok(t, "reload")
	if got := strings.TrimSpace(ok(t, "evaluate-js", "typeof window.marker").stdout); got != "undefined" {
		t.Errorf("window state survived reload: typeof marker = %q", got)
	}
	ok(t, "reload", "--hard")
}
