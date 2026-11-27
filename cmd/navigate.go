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
		tabCtx, _ = s.NewTab()
		if err := RunBounded(tabCtx, browser.SelectCurrentTab()); err != nil {
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
