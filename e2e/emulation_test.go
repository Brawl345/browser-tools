package e2e

import (
	"testing"
)

func TestResizeSurvivesCommandsAndReloads(t *testing.T) {
	open(t, "/emulation.html")
	contains(t, ok(t, "resize", "800", "600"), "emulating 800x600")
	equal(t, "viewport", js(t, `innerWidth + "x" + innerHeight`), "800x600")
	ok(t, "reload")
	equal(t, "viewport after reload", js(t, `innerWidth + "x" + innerHeight`), "800x600")
	ok(t, "navigate", pageURL("/blank.html"))
	equal(t, "viewport after navigate", js(t, `innerWidth + "x" + innerHeight`), "800x600")

	contains(t, ok(t, "resize", "--reset"), "emulation cleared")
	if js(t, "innerWidth") == "800" {
		t.Error("the viewport is still 800 wide after --reset")
	}
}

func TestMobileClickIsATap(t *testing.T) {
	open(t, "/emulation.html")
	ok(t, "mouse", "click", "#tap")
	equal(t, "events of a mouse click", text(t, "#out"), "pointerdown:mouse mousedown click:mouse")

	ok(t, "resize", "--mobile", "--dpr", "2", "393", "852")
	ok(t, "reload")
	equal(t, "mobile environment", js(t, `[innerWidth, navigator.maxTouchPoints > 0, matchMedia("(pointer: coarse)").matches, devicePixelRatio].join(" ")`), "980 true true 2")
	ok(t, "mouse", "click", "#tap")
	equal(t, "events of a tap", text(t, "#out"), "pointerdown:touch touchstart mousedown click:touch")
}

func TestColorScheme(t *testing.T) {
	open(t, "/emulation.html")
	dark := `matchMedia("(prefers-color-scheme: dark)").matches`
	ok(t, "resize", "--color-scheme", "dark")
	equal(t, "dark", js(t, dark), "true")
	ok(t, "resize", "--color-scheme", "light")
	equal(t, "dark after light", js(t, dark), "false")
	ok(t, "resize", "--color-scheme", "dark")
	ok(t, "resize", "--reset")
	equal(t, "dark after --reset", js(t, dark), "false")
}
