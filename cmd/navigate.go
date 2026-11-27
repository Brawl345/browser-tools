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

	rawURL := normalizeURL(fs.Arg(0))

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

// opaqueSchemes are URL schemes written without "//".
var opaqueSchemes = []string{"about:", "data:", "blob:", "javascript:", "view-source:", "mailto:"}

// normalizeURL adds a scheme to a bare host: http for local addresses, https
// for everything else.
func normalizeURL(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	lower := strings.ToLower(raw)
	for _, s := range opaqueSchemes {
		if strings.HasPrefix(lower, s) {
			return raw
		}
	}
	host := strings.SplitN(strings.SplitN(lower, "/", 2)[0], ":", 2)[0]
	if host == "localhost" || host == "127.0.0.1" ||strings.HasSuffix(host, ".localhost") {
		return "http://" + raw
	}
	return "https://" + raw
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
