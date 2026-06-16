package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Scroll(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("scroll", flag.ExitOnError)
	x := fs.Int("x", -1, "scroll to absolute X position (pixels)")
	y := fs.Int("y", -1, "scroll to absolute Y position (pixels)")
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
	if selector != "" {
		modes++
	}
	if hasPos {
		modes++
	}
	if *top {
		modes++
	}
	if *bottom {
		modes++
	}
	if modes == 0 {
		fs.Usage()
		os.Exit(1)
	}
	if modes > 1 {
		fmt.Fprintln(os.Stderr, "error: choose only one of <selector>, --x/--y, --top, --bottom")
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var action chromedp.Action
	var target string
	switch {
	case selector != "":
		action = chromedp.ScrollIntoView(selector, chromedp.ByQuery)
		target = selector
	case *top:
		action = chromedp.Evaluate(`window.scrollTo({left: 0, top: 0, behavior: "instant"})`, nil)
		target = "top"
	case *bottom:
		action = chromedp.Evaluate(`window.scrollTo({left: 0, top: document.documentElement.scrollHeight, behavior: "instant"})`, nil)
		target = "bottom"
	default:
		script := fmt.Sprintf(`window.scrollTo({left: %s, top: %s, behavior: "instant"})`,
			axisExpr(*x, "window.scrollX"), axisExpr(*y, "window.scrollY"))
		action = chromedp.Evaluate(script, nil)
		target = fmt.Sprintf("x=%d y=%d", max(*x, 0), max(*y, 0))
	}

	if err := chromedp.Run(tabCtx, action); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

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
