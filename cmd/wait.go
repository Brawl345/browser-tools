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

	var state string
	switch {
	case *hidden:
		state = "hidden"
	case *present:
		state = "present"
	case *absent:
		state = "absent"
	default:
		state = "visible"
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	// An XPath is polled: the element may only appear during the wait, so it
	// cannot be resolved to a CSS selector up front.
	run := func() error { return WaitXPath(ctx, tabCtx, sel, state) }
	if !IsXPath(sel) {
		var action chromedp.Action
		switch state {
		case "hidden":
			action = chromedp.WaitNotVisible(sel, chromedp.ByQuery)
		case "present":
			action = chromedp.WaitReady(sel, chromedp.ByQuery)
		case "absent":
			action = chromedp.WaitNotPresent(sel, chromedp.ByQuery)
		default:
			action = chromedp.WaitVisible(sel, chromedp.ByQuery)
		}
		run = func() error { return chromedp.Run(tabCtx, action) }
	}

	if err := run(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "%s✗%s timed out waiting for %s to be %s\n", ansiRed, ansiReset, sel, state)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s✓%s %s is %s\n", ansiGreen, ansiReset, sel, state)
}
