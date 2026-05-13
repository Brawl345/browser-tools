package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/domstorage"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func DOMStorage(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("dom-storage", flag.ExitOnError)
	localOnly := fs.Bool("local", false, "show localStorage only")
	sessionOnly := fs.Bool("session", false, "show sessionStorage only")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools dom-storage [--local | --session]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *localOnly && *sessionOnly {
		fmt.Fprintln(os.Stderr, "error: --local and --session are mutually exclusive")
		os.Exit(1)
	}

	showLocal := !*sessionOnly
	showSession := !*localOnly

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var currentURL string
	if err := chromedp.Run(tabCtx, chromedp.Location(&currentURL)); err != nil {
		fmt.Fprintf(os.Stderr, "error getting current URL: %v\n", err)
		os.Exit(1)
	}

	u, err := url.Parse(currentURL)
	if err != nil || u.Host == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine origin from URL %q\n", currentURL)
		os.Exit(1)
	}
	origin := u.Scheme + "://" + u.Host

	if err := chromedp.Run(tabCtx, domstorage.Enable()); err != nil {
		fmt.Fprintf(os.Stderr, "error enabling DOMStorage: %v\n", err)
		os.Exit(1)
	}

	printed := false

	fetch := func(isLocal bool) {
		label := "sessionStorage"
		if isLocal {
			label = "localStorage"
		}

		storageID := &domstorage.StorageID{
			SecurityOrigin: origin,
			IsLocalStorage: isLocal,
		}

		var items []domstorage.Item
		if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			items, err = domstorage.GetDOMStorageItems(storageID).Do(ctx)
			return err
		})); err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", label, err)
			return
		}

		if printed {
			fmt.Println()
		}
		printed = true

		fmt.Printf("%s%s%s  %s(%s)%s\n", ansiBold, label, ansiReset, ansiDim, origin, ansiReset)

		if len(items) == 0 {
			fmt.Printf("  %s(empty)%s\n", ansiDim, ansiReset)
			return
		}

		for _, item := range items {
			if len(item) < 2 {
				continue
			}
			key, val := item[0], item[1]
			fmt.Printf("  %s%s%s = %s\n", ansiBold, key, ansiReset, val)
		}
		fmt.Fprintf(os.Stderr, "  %s%d item(s)%s\n", ansiDim, len(items), ansiReset)
	}

	if showLocal {
		fetch(true)
	}
	if showSession {
		fetch(false)
	}
}
