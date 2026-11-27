package e2e

import (
	"strings"
	"testing"
)

func cookieCount(t *testing.T, name string) int {
	t.Helper()
	n := 0
	for _, l := range lines(ok(t, "cookie", "--all").stdout) {
		if l == name {
			n++
		}
	}
	return n
}

func TestCookieSetChangeDelete(t *testing.T) {
	open(t, "/blank.html")

	r := ok(t, "cookie", "set", "flavour", "vanilla")
	contains(t, r, "set cookie", "Value    : vanilla", "Path     : /", "Expires  : (session)")

	r = ok(t, "cookie", "set", "flavour", "chocolate", "--expires", "7d")
	contains(t, r, "changed cookie", "Value    : chocolate")
	lacks(t, r, "(session)")

	if got := strings.TrimSpace(ok(t, "evaluate-js", "document.cookie").stdout); got != "flavour=chocolate" {
		t.Errorf("document.cookie = %q", got)
	}
	if n := cookieCount(t, "flavour"); n != 1 {
		t.Errorf("%d cookies named flavour", n)
	}

	contains(t, ok(t, "cookie", "delete", "flavour"), "deleted flavour")
	contains(t, ok(t, "cookie"), "No cookies found.")
}

func TestCookieKeepsAttributesOfServerCookie(t *testing.T) {
	reset(t)
	ok(t, "navigate", pageURL("/set-cookie"))
	contains(t, ok(t, "cookie"), "server", "HttpOnly : true")

	r := ok(t, "cookie", "set", "server", "2")
	contains(t, r, "changed cookie", "Value    : 2", "HttpOnly : true")
	if n := cookieCount(t, "server"); n != 1 {
		t.Errorf("%d cookies named server after the change", n)
	}

	r = ok(t, "cookie", "set", "server", "3", "--http-only=false")
	contains(t, r, "HttpOnly : false")
	contains(t, ok(t, "evaluate-js", "document.cookie"), "server=3")
}

func TestCookieHostOnlyStaysSingle(t *testing.T) {
	open(t, "/blank.html")
	ok(t, "evaluate-js", "document.cookie = 'page=1; path=/'")
	ok(t, "cookie", "set", "page", "2")
	if n := cookieCount(t, "page"); n != 1 {
		t.Errorf("%d cookies named page after the change", n)
	}
	contains(t, ok(t, "evaluate-js", "document.cookie"), "page=2")
}

func TestCookieDomainAndPath(t *testing.T) {
	open(t, "/blank.html")
	ok(t, "evaluate-js", "document.cookie = 'p=root; path=/'; document.cookie = 'p=sub; path=/sub'")

	contains(t, ok(t, "cookie", "set", "p", "changed", "--path", "/sub"), "Path     : /sub", "Value    : changed")
	contains(t, ok(t, "evaluate-js", "document.cookie"), "p=root")

	ok(t, "cookie", "set", "other", "1", "--domain", "example.org")
	contains(t, ok(t, "cookie", "--all"), "example.org")
	contains(t, ok(t, "cookie", "delete", "other", "--domain", "example.org"), "deleted other")
}

func TestCookieErrors(t *testing.T) {
	open(t, "/blank.html")
	contains(t, fails(t, "cookie", "delete", "missing"), "no cookie named missing")
	contains(t, fails(t, "cookie", "set", "a", "1", "--expires", "soon"), `cannot read "soon"`)
	contains(t, fails(t, "cookie", "set", "a", "1", "--same-site", "Sometimes"), "Strict, Lax or None")

	ok(t, "navigate", "about:blank")
	contains(t, fails(t, "cookie", "set", "a", "1"), "pass --domain")
}
