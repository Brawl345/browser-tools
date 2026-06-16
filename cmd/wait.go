package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Wait(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	visible := fs.Bool("visible", false, "wait until the element is visible (default)")
	hidden := fs.Bool("hidden", false, "wait until the element is present but not visible")
	present := fs.Bool("present", false, "wait until the element exists in the DOM")
	absent := fs.Bool("absent", false, "wait until the element is gone from the DOM")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools wait [--visible | --hidden | --present | --absent] <selector>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Waits for an element to reach a state, then exits. Times out after --timeout.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}
	sel := fs.Arg(0)

	modes := 0
	for _, m := range []bool{*visible, *hidden, *present, *absent} {
		if m {
			modes++
		}
	}
	if modes > 1 {
		fmt.Fprintln(os.Stderr, "error: choose only one of --visible, --hidden, --present, --absent")
		os.Exit(1)
	}

	var action chromedp.Action
	var state string
	switch {
	case *hidden:
		action = chromedp.WaitNotVisible(sel, chromedp.ByQuery)
		state = "hidden"
	case *present:
		action = chromedp.WaitReady(sel, chromedp.ByQuery)
		state = "present"
	case *absent:
		action = chromedp.WaitNotPresent(sel, chromedp.ByQuery)
		state = "absent"
	default:
		action = chromedp.WaitVisible(sel, chromedp.ByQuery)
		state = "visible"
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	if err := chromedp.Run(tabCtx, action); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "%s✗%s timed out waiting for %s to be %s\n", ansiRed, ansiReset, sel, state)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s✓%s %s is %s\n", ansiGreen, ansiReset, sel, state)
}
