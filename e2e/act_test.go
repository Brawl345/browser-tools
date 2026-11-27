package e2e

import (
	"testing"
)

func attr(t *testing.T, sel, name string) string {
	t.Helper()
	l := lines(ok(t, "element", sel, "--attr", name).stdout)
	if len(l) == 0 {
		return ""
	}
	return l[0]
}

func value(t *testing.T, sel string) string {
	t.Helper()
	return attr(t, sel, "value")
}

func TestWaitForLateElement(t *testing.T) {
	open(t, "/wait.html")
	equal(t, "#late", text(t, "#late"), "arrived")
}

func TestWaitStates(t *testing.T) {
	open(t, "/wait.html")
	contains(t, ok(t, "wait", "#spinner", "--absent"), "#spinner is absent")
	contains(t, ok(t, "wait", "#toast", "--hidden"), "#toast is hidden")
	contains(t, ok(t, "wait", "h1", "--present"), "h1 is present")
	contains(t, ok(t, "wait", "h1"), "h1 is visible")
	contains(t, fails(t, "--timeout", "1s", "wait", "h1", "--hidden"), "did not become hidden")
}

func TestWaitOnPageThatPollsForever(t *testing.T) {
	open(t, "/poll.html")
	contains(t, ok(t, "--timeout", "3s", "wait"), "page is idle")
}

func TestLazySectionIsFoundByTheScan(t *testing.T) {
	open(t, "/lazy.html")
	ok(t, "mouse", "click", "#lazy")
	equal(t, "#lazy text", text(t, "#lazy"), "Load more")
}

func TestScanRestoresScrollOnFailure(t *testing.T) {
	open(t, "/lazy.html")
	ok(t, "scroll", "--y", "300")
	contains(t, fails(t, "--timeout", "3s", "mouse", "click", "#nope"), "the whole page was scrolled through")
	equal(t, "scrollY after the failed search", js(t, "scrollY"), "300")
}

func TestFill(t *testing.T) {
	open(t, "/form.html")
	ok(t, "fill", "#q", "new")
	equal(t, "#q after fill", value(t, "#q"), "oldnew")
	ok(t, "fill", "#q", "fresh", "--clear")
	equal(t, "#q after fill --clear", value(t, "#q"), "fresh")
	ok(t, "fill", "#note", "line")
	equal(t, "#note", value(t, "#note"), "line")
}

func TestCheck(t *testing.T) {
	open(t, "/form.html")
	contains(t, ok(t, "check", "#terms"), "checked: #terms")
	equal(t, "#terms checked", attr(t, "#terms", "checked"), "true")
	contains(t, ok(t, "check", "#terms", "--uncheck"), "unchecked: #terms")
	equal(t, "#terms checked", attr(t, "#terms", "checked"), "false")
	ok(t, "check", "#r2")
	equal(t, "#r2 checked", attr(t, "#r2", "checked"), "true")
	contains(t, fails(t, "check", "#r2", "--uncheck"), "a radio button cannot be unchecked")
}

func TestSelect(t *testing.T) {
	open(t, "/form.html")
	contains(t, ok(t, "select", "#color", "g"), `selected: "Green" (value="g", index=1)`)
	equal(t, "#out after change", text(t, "#out"), "color g")
	ok(t, "select", "#color", "Blue", "--by-label")
	equal(t, "#color", value(t, "#color"), "b")
	ok(t, "select", "#color", "0", "--by-index")
	equal(t, "#color", value(t, "#color"), "r")
}

func TestKey(t *testing.T) {
	open(t, "/form.html")
	ok(t, "evaluate-js", "q.value = ''")
	ok(t, "key", "a", "--selector", "#q")
	equal(t, "#q", value(t, "#q"), "a")
	ok(t, "key", "Escape")
	ok(t, "key", "Enter", "--selector", "#q")
	equal(t, "#out after Enter", text(t, "#out"), "submitted a")
	equal(t, "keys seen", text(t, "#keys"), "a Escape Enter")
}

func TestDrag(t *testing.T) {
	open(t, "/form.html")
	ok(t, "mouse", "drag", "#drag", "--to", "#drop")
	equal(t, "#out after drag", text(t, "#out"), "dropped")
}

func TestScroll(t *testing.T) {
	open(t, "/form.html")
	ok(t, "scroll", "#bottom")
	equal(t, "#bottom in view", js(t, "document.getElementById('bottom').getBoundingClientRect().top < innerHeight"), "true")
	ok(t, "scroll", "--top")
	equal(t, "scrollY after --top", js(t, "scrollY"), "0")
	ok(t, "scroll", "--y", "500")
	equal(t, "scrollY after --y 500", js(t, "scrollY"), "500")
	ok(t, "scroll", "--bottom")
	equal(t, "at the bottom", js(t, "Math.ceil(scrollY + innerHeight) >= document.documentElement.scrollHeight"), "true")
}
