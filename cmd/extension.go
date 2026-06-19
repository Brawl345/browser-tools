package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/extensions"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Extension(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("extension", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools extension <command> [args]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  load <dir>      Load an unpacked extension, printing its ID")
		fmt.Fprintln(os.Stderr, "  list            List loaded unpacked extensions")
		fmt.Fprintln(os.Stderr, "  reload <id>     Reload a loaded extension from its source directory")
		fmt.Fprintln(os.Stderr, "  uninstall <id>  Remove a loaded extension")
		fmt.Fprintln(os.Stderr, "  action <id>     Trigger the extension's toolbar action on the active tab")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Requires a Chrome for Testing build: --browser cft-stable|cft-beta|cft-dev|cft-canary")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if !browser.IsChromeForTesting(variant) {
		fmt.Fprintf(os.Stderr, "error: extensions require Chrome for Testing — pass --browser cft-stable (install it with 'browser-tools update-cft')\n")
		os.Exit(1)
	}

	rest := fs.Args()
	if len(rest) < 1 {
		fs.Usage()
		os.Exit(1)
	}
	sub, rest := rest[0], rest[1:]

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	allocCtx, _ := browser.Connect(ctx, port)

	switch sub {
	case "load":
		if len(rest) < 1 {
			fs.Usage()
			os.Exit(1)
		}
		extLoad(allocCtx, rest[0])
	case "list":
		extList(allocCtx)
	case "reload":
		if len(rest) < 1 {
			fs.Usage()
			os.Exit(1)
		}
		extReload(allocCtx, rest[0])
	case "uninstall":
		if len(rest) < 1 {
			fs.Usage()
			os.Exit(1)
		}
		extUninstall(allocCtx, rest[0])
	case "action":
		if len(rest) < 1 {
			fs.Usage()
			os.Exit(1)
		}
		extAction(allocCtx, rest[0])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub)
		fs.Usage()
		os.Exit(1)
	}
}

func extLoad(allocCtx context.Context, dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(filepath.Join(abs, "manifest.json")); err != nil {
		fmt.Fprintf(os.Stderr, "error: no manifest.json in %s\n", abs)
		os.Exit(1)
	}

	bootstrapCtx, cancel := browser.NewTab(allocCtx)
	defer cancel()

	var id string
	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		var e error
		id, e = extensions.LoadUnpacked(abs).Do(cdp.WithExecutor(ctx, c.Browser))
		return e
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error loading extension: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s✓%s loaded %s\n", ansiGreen, ansiReset, id)
}

func extReload(allocCtx context.Context, id string) {
	bootstrapCtx, cancel := browser.NewTab(allocCtx)
	defer cancel()

	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		ex := cdp.WithExecutor(ctx, c.Browser)
		exts, err := extensions.GetExtensions().Do(ex)
		if err != nil {
			return err
		}
		path := ""
		for _, e := range exts {
			if e.ID == id {
				path = e.Path
			}
		}
		if path == "" {
			return fmt.Errorf("no loaded extension with id %s", id)
		}
		_, err = extensions.LoadUnpacked(path).Do(ex)
		return err
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error reloading extension: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s✓%s reloaded %s\n", ansiGreen, ansiReset, id)
}

func extList(allocCtx context.Context) {
	bootstrapCtx, cancel := browser.NewTab(allocCtx)
	defer cancel()

	var exts []*extensions.ExtensionInfo
	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		var e error
		exts, e = extensions.GetExtensions().Do(cdp.WithExecutor(ctx, c.Browser))
		return e
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error listing extensions: %v\n", err)
		os.Exit(1)
	}

	if len(exts) == 0 {
		fmt.Println("No unpacked extensions loaded.")
		return
	}
	for _, e := range exts {
		status := ansiGreen + "enabled" + ansiReset
		if !e.Enabled {
			status = ansiYellow + "disabled" + ansiReset
		}
		fmt.Printf("%s  %-30s  %-8s  %s\n", e.ID, truncate(e.Name, 30), e.Version, status)
	}
}

func extUninstall(allocCtx context.Context, id string) {
	bootstrapCtx, cancel := browser.NewTab(allocCtx)
	defer cancel()

	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return extensions.Uninstall(id).Do(cdp.WithExecutor(ctx, c.Browser))
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error uninstalling extension: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s✓%s uninstalled %s\n", ansiGreen, ansiReset, id)
}

func extAction(allocCtx context.Context, id string) {
	bootstrapCtx, cancel := browser.NewTab(allocCtx)
	defer cancel()

	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(context.Context) error { return nil })); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	bootstrapID := string(chromedp.FromContext(bootstrapCtx).Target.TargetID)
	activeURL := ""
	if pageID := browser.ActiveTabID(allocCtx, bootstrapID); pageID != "" {
		if tabs, err := browser.GetPageTabs(bootstrapCtx, bootstrapID); err == nil {
			for _, t := range tabs {
				if t.ID == pageID {
					activeURL = t.URL
				}
			}
		}
	}

	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		ex := cdp.WithExecutor(ctx, c.Browser)
		tabs, err := target.GetTargets().WithFilter(target.Filter{{Type: "tab"}}).Do(ex)
		if err != nil {
			return err
		}
		if len(tabs) == 0 {
			return fmt.Errorf("no tab to trigger the action on")
		}
		chosen := tabs[0].TargetID
		for _, t := range tabs {
			if activeURL != "" && t.URL == activeURL {
				chosen = t.TargetID
				break
			}
		}
		return extensions.TriggerAction(id, string(chosen)).Do(ex)
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error triggering action: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s✓%s triggered action for %s\n", ansiGreen, ansiReset, id)
}
