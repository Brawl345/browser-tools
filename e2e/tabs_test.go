package e2e

import (
	"strings"
	"testing"
	"time"
)

func currentTab(t *testing.T) string {
	t.Helper()
	for _, l := range lines(ok(t, "tab").stdout) {
		if strings.HasPrefix(l, "▶") {
			return l
		}
	}
	t.Fatal("no tab is marked current")
	return ""
}

func TestTabsOpenedByThePageTakeOver(t *testing.T) {
	for _, c := range []struct{ button, via string }{
		{"#link", "link"},
		{"#tab", "open"},
		{"#popup", "popup"},
	} {
		t.Run(c.via, func(t *testing.T) {
			open(t, "/tabs.html")
			ok(t, "mouse", "click", c.button)
			equal(t, "location.search in the new tab", js(t, "location.search"), "?via="+c.via)
			if n := tabCount(t); n != 2 {
				t.Errorf("%d tabs open, want 2", n)
			}
			contains(t, result{stdout: currentTab(t)}, "Closer")

			ok(t, "mouse", "click", "#close")
			equal(t, "pathname after closing", js(t, "location.pathname"), "/tabs.html")
			if n := tabCount(t); n != 1 {
				t.Errorf("%d tabs open after closing, want 1", n)
			}
		})
	}
}

func TestTabClosedByItsOwnClick(t *testing.T) {
	for _, button := range []string{"#close", "#late-close"} {
		t.Run(button, func(t *testing.T) {
			open(t, "/tabs.html")
			ok(t, "mouse", "click", "#tab")
			r := run(t, "mouse", "click", button)
			if r.code != 0 && !strings.Contains(r.output(), "closed") {
				t.Errorf("exit %d without naming the closed tab:\n%s", r.code, r.output())
			}
			equal(t, "pathname after the tab closed itself", js(t, "location.pathname"), "/tabs.html")
		})
	}
}

func TestClickOnATabThatClosesBeforeTheRelease(t *testing.T) {
	open(t, "/tabs.html")
	ok(t, "mouse", "click", "#tab")
	contains(t, ok(t, "mouse", "click", "#press-close"), "the tab closed itself")
	equal(t, "pathname after the tab closed itself", js(t, "location.pathname"), "/tabs.html")
}

func TestTabActivateSticks(t *testing.T) {
	open(t, "/list.html")
	ok(t, "navigate", "--new-tab", pageURL("/blank.html"))
	equal(t, "title in the new tab", js(t, "document.title"), "Blank")

	ok(t, "tab", "--activate", "1")
	for range 2 {
		equal(t, "title after --activate 1", js(t, "document.title"), "Products")
	}
	contains(t, result{stdout: currentTab(t)}, "Products")
}

func TestBackgroundTabDoesNotTakeOver(t *testing.T) {
	open(t, "/list.html")
	cdpBrowser(t, "Target.createTarget", map[string]any{"url": pageURL("/blank.html?background"), "background": true})
	deadline := time.Now().Add(3 * time.Second)
	for tabCount(t) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("the background tab never showed up")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for range 2 {
		equal(t, "title with a background tab open", js(t, "document.title"), "Products")
	}
}

func TestClosingTheLastTab(t *testing.T) {
	reset(t)
	ok(t, "tab", "--close", "1")
	contains(t, ok(t, "tab"), "No tabs open.")

	ok(t, "navigate", pageURL("/list.html"))
	equal(t, "title in the new window", js(t, "document.title"), "Products")
	if n := tabCount(t); n != 1 {
		t.Errorf("%d tabs open, want 1", n)
	}
}

func TestElementInAnotherTab(t *testing.T) {
	open(t, "/selectors.html")
	ok(t, "navigate", "--new-tab", pageURL("/list.html"))

	r := fails(t, "--timeout", "3s", "mouse", "click", "#save")
	contains(t, r, "nothing matches #save", `"#save" matches in tab 1 (Selectors): run `+"`browser-tools tab --activate 1`")

	r = fails(t, "--timeout", "3s", "mouse", "click", "Feed suchen")
	contains(t, r, `"text=Feed suchen" matches in tab 1 (Selectors)`)
}

func TestElementInATabOpenedDuringTheWait(t *testing.T) {
	open(t, "/tabs.html")
	ok(t, "mouse", "click", "#delayed")
	equal(t, "pathname right after the click", js(t, "location.pathname"), "/tabs.html")
	equal(t, "text in the tab opened during the wait", text(t, "#closer"), "Closer")
}
