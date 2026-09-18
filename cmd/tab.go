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
	activateIdx := fs.Int("activate", 0, "activate the tab with this index")
	closeIdx := fs.Int("close", 0, "close the tab with this index")
	refreshIdx := fs.Int("refresh", 0, "reload the tab with this index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools tab [--activate <n>] [--close <n>] [--refresh <n>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	s := MustSession(ctx, variant, port)
	tabs, err := s.Tabs()
	if err != nil {
		fail("cannot list tabs: %v", err)
	}

	switch {
	case *closeIdx > 0:
		t := mustTabAt(tabs, *closeIdx)
		if err := s.CloseTabByID(t.ID); err != nil {
			fail("cannot close the tab: %v", err)
		}
		return
	case *refreshIdx > 0:
		t := mustTabAt(tabs, *refreshIdx)
		if err := chromedp.Run(s.Tab(browser.TargetID(t.ID)), chromedp.Reload()); err != nil {
			fail("cannot reload the tab: %v", err)
		}
		return
	case *activateIdx > 0:
		t := mustTabAt(tabs, *activateIdx)
		if err := s.ActivateTabByID(t.ID); err != nil {
			fail("cannot activate the tab: %v", err)
		}
		return
	}

	if len(tabs) == 0 {
		fmt.Println("No tabs open.")
		return
	}

	var activeID string
	if t, err := s.ActiveTarget(); err == nil && t != nil {
		activeID = string(t.TargetID)
	}
	for i, t := range tabs {
		marker := "  "
		if t.ID == activeID {
			marker = ansiGreen + "▶" + ansiReset + " "
		}
		fmt.Printf("%s%2d  %-45s  %s\n", marker, i+1, truncate(t.Title, 45), t.URL)
	}
}

func mustTabAt(tabs []browser.TabInfo, idx int) browser.TabInfo {
	i := idx - 1
	if i < 0 || i >= len(tabs) {
		fail("there is no tab %d (%d tab(s) open)", idx, len(tabs))
	}
	return tabs[i]
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
