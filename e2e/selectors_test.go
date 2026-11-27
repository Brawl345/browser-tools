package e2e

import (
	"strings"
	"testing"
)

func TestTextSelectors(t *testing.T) {
	cases := []struct{ sel, clicked string }{
		{"text=Save", "clicked save"},
		{"text=save draft", "clicked save-draft"},
		{"text=Inner label", "clicked inner"},
		{"text=Install now", "clicked install"},
		{"text=Close panel", "clicked icon"},
		{"#toolbar-b >> text=Edit", "clicked edit-b"},
		{"#toolbar-b text=Edit", "clicked edit-b"},
		{"#shadow-btn", "clicked shadow-btn"},
		{"text=Shadow action", "clicked shadow-btn"},
		{"host-el >>> button", "clicked shadow-btn"},
	}
	open(t, "/selectors.html")
	for _, c := range cases {
		t.Run(c.sel, func(t *testing.T) {
			ok(t, "evaluate-js", "out.textContent = ''")
			ok(t, "mouse", "click", c.sel)
			equal(t, "#out", text(t, "#out"), c.clicked)
		})
	}
}

func TestTextSelectorPicksInnermost(t *testing.T) {
	open(t, "/selectors.html")
	got := strings.Join(lines(ok(t, "element", "text=Inner label", "--attr", "id").stdout), ",")
	equal(t, "ids matching text=Inner label", got, "inner")
}

func TestPlaceholderSelector(t *testing.T) {
	open(t, "/selectors.html")
	ok(t, "fill", "text=E-Mail address", "me@example.com")
	equal(t, "#email value", value(t, "#email"), "me@example.com")
}

func TestXPathSelectors(t *testing.T) {
	open(t, "/selectors.html")
	equal(t, "//h1", text(t, "//h1"), "Selectors")
	contains(t, fails(t, "element", "//h1/text()"), "not elements", "drop a trailing /text() or /@attr")
	contains(t, fails(t, "element", "//a/@href"), "not elements")
}

func TestHiddenElementReport(t *testing.T) {
	open(t, "/selectors.html")
	// The real mouse pointer resting over the test window would hover a row.
	ok(t, "mouse", "hover", "h1")
	r := fails(t, "--timeout", "2s", "mouse", "click", "#delete")
	contains(t, r,
		"#delete matches 1 element(s) but none is visible",
		"hidden by its ancestor div.actions (display:none)",
		"it shows on hover: run `browser-tools mouse hover '#card-3'` first",
		"--force acts on it through JavaScript",
	)

	r = fails(t, "--timeout", "2s", "mouse", "click", "#tw")
	contains(t, r, "visibility:hidden", "it shows on hover: run `browser-tools mouse hover '#row-7'` first")

	ok(t, "mouse", "hover", "#card-3")
	ok(t, "mouse", "click", "#delete")
	equal(t, "#out after hover and click", text(t, "#out"), "clicked delete")
}

func TestForceHintOnlyWhereForceExists(t *testing.T) {
	open(t, "/selectors.html")
	contains(t, fails(t, "--timeout", "2s", "check", "#delete"), "--force acts on it")
	lacks(t, fails(t, "--timeout", "2s", "wait", "#delete"), "--force")

	open(t, "/form.html")
	contains(t, fails(t, "--timeout", "2s", "select", "#hidden-select", "a"), "--force acts on it")
	lacks(t, fails(t, "--timeout", "2s", "fill", "#hidden-input", "x"), "--force")
}

func TestForceActsOnHiddenElement(t *testing.T) {
	open(t, "/selectors.html")
	ok(t, "mouse", "click", "#delete", "--force")
	equal(t, "#out after a forced click", text(t, "#out"), "clicked delete")
}

func TestCoveredElement(t *testing.T) {
	open(t, "/selectors.html")
	contains(t, ok(t, "mouse", "click", "#covered"), "div#overlay lies on top and takes the event")
	equal(t, "#out", text(t, "#out"), "clicked overlay")
}

func TestProseSelectorSuggestsText(t *testing.T) {
	open(t, "/selectors.html")
	r := fails(t, "--timeout", "2s", "mouse", "click", "Feed suchen")
	contains(t, r, "nothing matches Feed suchen", `as CSS this looks for a <Feed> tag; "text=Feed suchen" matches 1 element(s) here`)
}

func TestMissingTextListsPageTexts(t *testing.T) {
	open(t, "/selectors.html")
	r := fails(t, "--timeout", "2s", "mouse", "click", "text=Nothing like it")
	contains(t, r, "nothing matches text=Nothing like it", "texts on the page:", `"Save draft"`)
}

func TestCSSReportNamesClosestPrefix(t *testing.T) {
	open(t, "/selectors.html")
	r := fails(t, "--timeout", "2s", "element", ".toolbar .missing")
	contains(t, r, `closest hit: ".toolbar" matches 2 element(s)`)
}

func TestFailureReportsBeatTheWatchdog(t *testing.T) {
	open(t, "/selectors.html")
	ok(t, "navigate", "--new-tab", pageURL("/lazy.html"))
	cases := []struct {
		args   []string
		report string
	}{
		{[]string{"mouse", "click", "#nothing"}, "nothing matches #nothing"},
		{[]string{"mouse", "click", "text=Nowhere"}, "nothing matches text=Nowhere"},
		{[]string{"mouse", "click", "#save"}, "matches in tab 1"},
		{[]string{"element", "//section[@id='none']"}, "nothing matches"},
		{[]string{"wait", "h1", "--absent"}, "did not become absent"},
		{[]string{"fill", "#nothing", "x"}, "nothing matches #nothing"},
	}
	for _, timeout := range []string{"1s", "3s"} {
		for _, c := range cases {
			r := fails(t, append([]string{"--timeout", timeout}, c.args...)...)
			contains(t, r, c.report)
		}
	}
}
