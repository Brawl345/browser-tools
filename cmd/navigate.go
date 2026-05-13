package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"browser-tools/browser"
	flag "github.com/spf13/pflag"
	"github.com/chromedp/chromedp"
)

func Navigate(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("navigate", flag.ExitOnError)
	newTab := fs.Bool("new-tab", false, "open URL in a new tab instead of reusing the current one")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools navigate [--new-tab] <url>")
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

	rawURL := fs.Arg(0)
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)

	var tabCtx context.Context
	if *newTab {
		tabCtx, _ = browser.NewTab(allocCtx)
	} else {
		tabCtx, _ = browser.ExistingOrNewTab(allocCtx)
	}

	if err := chromedp.Run(tabCtx, browser.ActivateCurrentTab()); err != nil {
		fmt.Fprintf(os.Stderr, "navigate error: %v\n", err)
		os.Exit(1)
	}
	if err := chromedp.Run(tabCtx, browser.StartNavigate(rawURL)); err != nil {
		fmt.Fprintf(os.Stderr, "navigate error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Navigated to %s\n", rawURL)
}
