package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Screenshot(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("screenshot", flag.ExitOnError)
	fullPage := fs.Bool("full-page", false, "capture the entire page, not just the viewport")
	selector := fs.String("selector", "", "capture only the element matching this CSS selector")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools screenshot [--full-page] [--selector <css>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *fullPage && *selector != "" {
		fmt.Fprintln(os.Stderr, "error: --full-page and --selector are mutually exclusive")
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var buf []byte
	var action chromedp.Action
	switch {
	case *selector != "":
		var count int
		countScript := fmt.Sprintf(`document.querySelectorAll(%s).length`, jsonStr(*selector))
		action = chromedp.Tasks{
			chromedp.WaitVisible(*selector, chromedp.ByQuery),
			chromedp.Evaluate(countScript, &count),
			chromedp.ActionFunc(func(context.Context) error {
				if count > 1 {
					fmt.Fprintf(os.Stderr, "note: %d elements match %q; capturing the first. Refine the selector to target a single element.\n", count, *selector)
				}
				return nil
			}),
			chromedp.Screenshot(*selector, &buf, chromedp.NodeVisible, chromedp.ByQuery),
		}
	case *fullPage:
		action = chromedp.FullScreenshot(&buf, 100)
	default:
		action = chromedp.CaptureScreenshot(&buf)
	}

	if err := chromedp.Run(tabCtx, action); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ts := time.Now().Format("20060102-150405")
	path := fmt.Sprintf("/tmp/screenshot-%s.png", ts)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error saving screenshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(path)
}
