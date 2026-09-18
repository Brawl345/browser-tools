package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Scroll(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("scroll", flag.ExitOnError)
	x := fs.Int("x", -1, "scroll to this absolute X position (pixels)")
	y := fs.Int("y", -1, "scroll to this absolute Y position (pixels)")
	top := fs.Bool("top", false, "scroll to the top of the page")
	bottom := fs.Bool("bottom", false, "scroll to the bottom of the page")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools scroll [<selector> | --x <n> --y <n> | --top | --bottom]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	selector := fs.Arg(0)
	hasPos := *x >= 0 || *y >= 0

	modes := 0
	for _, m := range []bool{selector != "", hasPos, *top, *bottom} {
		if m {
			modes++
		}
	}
	if modes == 0 {
		fs.Usage()
		os.Exit(1)
	}
	if modes > 1 {
		fail("choose only one of <selector>, --x/--y, --top, --bottom")
	}

	s := MustSession(ctx, variant, port)

	// Locate already scrolls the element into view.
	if selector != "" {
		m := Locate(ctx, s, selector, StatePresent)
		SettleAfter(m.Tab)()
		fmt.Printf("%s✓%s scrolled to %s%s\n", ansiGreen, ansiReset, selector, matchNote(m))
		return
	}

	tabCtx := MustTab(s)
	var script, target string
	switch {
	case *top:
		script, target = `window.scrollTo({left: 0, top: 0, behavior: "instant"})`, "top"
	case *bottom:
		script, target = `window.scrollTo({left: 0, top: document.documentElement.scrollHeight, behavior: "instant"})`, "bottom"
	default:
		script = fmt.Sprintf(`window.scrollTo({left: %s, top: %s, behavior: "instant"})`,
			axisExpr(*x, "window.scrollX"), axisExpr(*y, "window.scrollY"))
		target = fmt.Sprintf("x=%d y=%d", max(*x, 0), max(*y, 0))
	}

	settle := SettleAfter(tabCtx)
	if err := RunBounded(tabCtx, chromedp.Evaluate(script, nil)); err != nil {
		fail("%v", err)
	}

	settle()
	fmt.Printf("%s✓%s scrolled to %s\n", ansiGreen, ansiReset, target)
}

// axisExpr returns the JS expression for an axis: the pixel value if set
// (>= 0), otherwise the current scroll position so the axis stays put.
func axisExpr(v int, current string) string {
	if v < 0 {
		return current
	}
	return fmt.Sprintf("%d", v)
}
