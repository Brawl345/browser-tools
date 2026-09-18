package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Navigate(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("navigate", flag.ExitOnError)
	newTab := fs.Bool("new-tab", false, "open the URL in a new tab instead of reusing the current one")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools navigate [--new-tab] <url>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Waits for the page to load and go quiet before returning.")
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

	s := MustSession(ctx, variant, port)

	var tabCtx context.Context
	if *newTab {
		if s.OpenTabInForeground(rawURL) {
			reportNavigation(ctx, s, rawURL)
			return
		}
		tabCtx, _ = browser.NewTab(s.Alloc)
		if err := chromedp.Run(tabCtx, browser.ActivateCurrentTab()); err != nil {
			fail("%v", err)
		}
	} else {
		tabCtx = MustTab(s)
	}

	w := watch(tabCtx)
	if err := RunBounded(tabCtx, browser.StartNavigate(rawURL)); err != nil {
		fail("%v", err)
	}
	report("Navigated to", rawURL, WaitLoaded(ctx, tabCtx, w))
}

// reportNavigation waits on whichever tab ends up in front after a new-tab
// navigation, since window.open puts the page in a target this process does not
// hold yet.
func reportNavigation(ctx context.Context, s *browser.Session, url string) {
	tabCtx, err := s.ActiveTab()
	if err != nil || tabCtx == nil {
		fmt.Printf("Navigated to %s\n", url)
		return
	}
	report("Navigated to", url, WaitLoaded(ctx, tabCtx, watch(tabCtx)))
}

func report(verb, url string, loaded bool) {
	if loaded {
		fmt.Printf("%s %s\n", verb, url)
		return
	}
	fmt.Printf("%s %s (still loading — raise --timeout or use `wait <selector>`)\n", verb, url)
}

func Reload(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("reload", flag.ExitOnError)
	hard := fs.Bool("hard", false, "bypass the cache")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools reload [--hard]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	s := MustSession(ctx, variant, port)
	tabCtx := MustTab(s)

	w := watch(tabCtx)
	if err := RunBounded(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		return page.Reload().WithIgnoreCache(*hard).Do(c)
	})); err != nil {
		fail("%v", err)
	}

	var url string
	_ = RunBounded(tabCtx, chromedp.Evaluate(`location.href`, &url))
	report("Reloaded", url, WaitLoaded(ctx, tabCtx, w))
}
