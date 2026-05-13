package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"

	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Tab(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("tab", flag.ExitOnError)
	activateIdx := fs.Int("activate", 0, "activate tab by index")
	closeIdx := fs.Int("close", 0, "close tab by index")
	refreshIdx := fs.Int("refresh", 0, "refresh tab by index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools tab [--activate <n>] [--close <n>] [--refresh <n>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)

	// One bootstrap tab for the entire operation; its ID is excluded from the
	// tab list so it never appears in the output or shifts indices.
	bootstrapCtx, bootstrapCancel := browser.NewTab(allocCtx)
	defer bootstrapCancel()

	var tabs []browser.TabInfo
	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		var err error
		tabs, err = browser.GetPageTabs(ctx, string(c.Target.TargetID))
		return err
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error listing tabs: %v\n", err)
		os.Exit(1)
	}

	if *closeIdx > 0 {
		tab, ok := resolveTab(tabs, *closeIdx)
		if !ok {
			os.Exit(1)
		}
		if err := chromedp.Run(bootstrapCtx, browser.CloseTabByID(tab.ID)); err != nil {
			fmt.Fprintf(os.Stderr, "error closing tab: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *refreshIdx > 0 {
		tab, ok := resolveTab(tabs, *refreshIdx)
		if !ok {
			os.Exit(1)
		}
		// Intentionally no cancel: chromedp closes the target on cancel for
		// WithTargetID contexts. The cleanup goroutine blocks on <-ctx.Done()
		// which never fires (allocCtx derives from context.Background()), so
		// it is killed at process exit without calling CloseTarget.
		tabCtx, _ := chromedp.NewContext(allocCtx,
			chromedp.WithTargetID(browser.TargetID(tab.ID)),
			chromedp.WithLogf(func(string, ...any) {}),
			chromedp.WithErrorf(func(string, ...any) {}),
		)
		if err := chromedp.Run(tabCtx, chromedp.Reload()); err != nil {
			fmt.Fprintf(os.Stderr, "error refreshing tab: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *activateIdx > 0 {
		tab, ok := resolveTab(tabs, *activateIdx)
		if !ok {
			os.Exit(1)
		}
		if err := chromedp.Run(bootstrapCtx, browser.ActivateTabByID(tab.ID)); err != nil {
			fmt.Fprintf(os.Stderr, "error activating tab: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(tabs) == 0 {
		fmt.Println("No tabs open.")
		return
	}

	activeID := browser.ActiveTabID(allocCtx, string(chromedp.FromContext(bootstrapCtx).Target.TargetID))
	for i, t := range tabs {
		marker := "  "
		if t.ID == activeID {
			marker = ansiGreen + "▶" + ansiReset + " "
		}
		fmt.Printf("%s%2d  %-45s  %s\n", marker, i+1, truncate(t.Title, 45), t.URL)
	}
}

func resolveTab(tabs []browser.TabInfo, idx int) (browser.TabInfo, bool) {
	i := idx - 1
	if i < 0 || i >= len(tabs) {
		fmt.Fprintf(os.Stderr, "no tab at index %d (have %d tab(s))\n", idx, len(tabs))
		return browser.TabInfo{}, false
	}
	return tabs[i], true
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
