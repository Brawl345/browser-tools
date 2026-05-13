package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/domstorage"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Clear(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("clear", flag.ExitOnError)
	doCookies      := fs.Bool("cookies",        false, "clear cookies")
	doCache        := fs.Bool("cache",          false, "clear HTTP cache (always browser-wide)")
	doLocalStorage := fs.Bool("local-storage",  false, "clear localStorage for the current origin")
	doSession      := fs.Bool("session-storage",false, "clear sessionStorage for the current origin")
	doIndexedDB    := fs.Bool("indexeddb",       false, "clear IndexedDB for the current origin")
	doCacheStorage := fs.Bool("cache-storage",  false, "clear Cache API / service worker caches for the current origin")
	doServiceWorkers := fs.Bool("service-workers", false, "unregister service workers for the current origin")
	doAll          := fs.Bool("all",             false, "clear everything (shorthand for all flags above)")
	allOrigins     := fs.Bool("all-origins",    false, "for cookies: clear from all origins; for cache: already global; storage is always per-origin")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools clear [--cookies] [--cache] [--local-storage] [--session-storage] [--indexeddb] [--all] [--all-origins]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if !*doCookies && !*doCache && !*doLocalStorage && !*doSession && !*doIndexedDB && !*doCacheStorage && !*doServiceWorkers && !*doAll {
		fs.Usage()
		os.Exit(1)
	}

	if *doAll {
		*doCookies = true
		*doCache = true
		*doLocalStorage = true
		*doSession = true
		*doIndexedDB = true
		*doCacheStorage = true
		*doServiceWorkers = true
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var currentURL, origin string
	if *doLocalStorage || *doSession || *doIndexedDB || *doCacheStorage || *doServiceWorkers || (*doCookies && !*allOrigins) {
		if err := chromedp.Run(tabCtx, chromedp.Location(&currentURL)); err != nil {
			fmt.Fprintf(os.Stderr, "error getting current URL: %v\n", err)
			os.Exit(1)
		}
		if u, err := url.Parse(currentURL); err == nil && u.Host != "" {
			origin = u.Scheme + "://" + u.Host
		}
	}

	ok := true

	clearStorage := func(label string, types string) {
		if origin == "" {
			fmt.Fprintf(os.Stderr, "  %sskip %s — no valid origin%s\n", ansiDim, label, ansiReset)
			return
		}
		err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return storage.ClearDataForOrigin(origin, types).Do(ctx)
		}))
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error clearing %s: %v\n", label, err)
			ok = false
			return
		}
		fmt.Printf("  %s✓%s %s  %s(%s)%s\n", ansiGreen, ansiReset, label, ansiDim, origin, ansiReset)
	}

	if *doCookies {
		if *allOrigins {
			err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
				return storage.ClearCookies().Do(ctx)
			}))
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error clearing cookies: %v\n", err)
				ok = false
			} else {
				fmt.Printf("  %s✓%s cookies  %s(all origins)%s\n", ansiGreen, ansiReset, ansiDim, ansiReset)
			}
		} else {
			// fetch cookies for current URL, delete each by name+URL
			var cookies []*network.Cookie
			if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				cookies, err = network.GetCookies().WithURLs([]string{currentURL}).Do(ctx)
				return err
			})); err != nil {
				fmt.Fprintf(os.Stderr, "  error fetching cookies: %v\n", err)
				ok = false
			} else if len(cookies) == 0 {
				fmt.Printf("  %s✓%s cookies  %s(none found for %s)%s\n", ansiGreen, ansiReset, ansiDim, origin, ansiReset)
			} else {
				for _, c := range cookies {
					_ = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
						return network.DeleteCookies(c.Name).WithURL(currentURL).Do(ctx)
					}))
				}
				fmt.Printf("  %s✓%s cookies  %s(%d deleted for %s)%s\n", ansiGreen, ansiReset, ansiDim, len(cookies), origin, ansiReset)
			}
		}
	}

	if *doCache {
		err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return network.ClearBrowserCache().Do(ctx)
		}))
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error clearing cache: %v\n", err)
			ok = false
		} else {
			fmt.Printf("  %s✓%s HTTP cache  %s(browser-wide)%s\n", ansiGreen, ansiReset, ansiDim, ansiReset)
		}
	}

	if *doLocalStorage {
		clearStorage("localStorage", "local_storage")
	}
	if *doSession {
		if origin == "" {
			fmt.Fprintf(os.Stderr, "  %sskip sessionStorage — no valid origin%s\n", ansiDim, ansiReset)
		} else {
			err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
				if err := domstorage.Enable().Do(ctx); err != nil {
					return err
				}
				return domstorage.Clear(&domstorage.StorageID{
					SecurityOrigin: origin,
					IsLocalStorage: false,
				}).Do(ctx)
			}))
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error clearing sessionStorage: %v\n", err)
				ok = false
			} else {
				fmt.Printf("  %s✓%s sessionStorage  %s(%s)%s\n", ansiGreen, ansiReset, ansiDim, origin, ansiReset)
			}
		}
	}
	if *doIndexedDB {
		clearStorage("IndexedDB", "indexeddb")
	}
	if *doCacheStorage {
		clearStorage("Cache API", "cache_storage")
	}
	if *doServiceWorkers {
		clearStorage("service workers", "service_workers")
	}

	if !ok {
		os.Exit(1)
	}
}
