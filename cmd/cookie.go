package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Cookie(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("cookie", flag.ExitOnError)
	all := fs.Bool("all", false, "list cookies from all origins (not just the current tab's URL)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools cookie [--all]")
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
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var cookies []*network.Cookie
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		if *all {
			cookies, err = storage.GetCookies().Do(ctx)
		} else {
			var currentURL string
			if err = chromedp.Location(&currentURL).Do(ctx); err != nil {
				return err
			}
			cookies, err = network.GetCookies().WithURLs([]string{currentURL}).Do(ctx)
		}
		return err
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(cookies) == 0 {
		fmt.Println("No cookies found.")
		return
	}

	for i, c := range cookies {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s%s%s\n", ansiBold, c.Name, ansiReset)
		fmt.Printf("  Value    : %s\n", c.Value)
		fmt.Printf("  Domain   : %s\n", c.Domain)
		fmt.Printf("  Path     : %s\n", c.Path)

		if c.Session || c.Expires < 0 {
			fmt.Printf("  Expires  : (session)\n")
		} else {
			exp := time.Unix(int64(math.Round(c.Expires)), 0)
			fmt.Printf("  Expires  : %s\n", exp.UTC().Format(time.RFC1123))
		}

		fmt.Printf("  Secure   : %v\n", c.Secure)
		fmt.Printf("  HttpOnly : %v\n", c.HTTPOnly)

		if c.SameSite != "" {
			fmt.Printf("  SameSite : %s\n", c.SameSite)
		}
	}

	fmt.Fprintf(os.Stderr, "\n%s%d cookie(s)%s\n", ansiDim, len(cookies), ansiReset)
}
