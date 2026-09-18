package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/extensions"
	"github.com/chromedp/cdproto/target"
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
		fail("extensions require Chrome for Testing — pass --browser cft-stable (install it with 'browser-tools update-cft')")
	}

	rest := fs.Args()
	if len(rest) < 1 {
		fs.Usage()
		os.Exit(1)
	}
	sub, rest := rest[0], rest[1:]

	needsArg := func() string {
		if len(rest) < 1 {
			fs.Usage()
			os.Exit(1)
		}
		return rest[0]
	}

	s := MustSession(ctx, variant, port)

	switch sub {
	case "load":
		extLoad(s, needsArg())
	case "list":
		extList(s)
	case "reload":
		extReload(s, needsArg())
	case "uninstall":
		extUninstall(s, needsArg())
	case "action":
		extAction(s, needsArg())
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub)
		fs.Usage()
		os.Exit(1)
	}
}

func extLoad(s *browser.Session, dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fail("%v", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "manifest.json")); err != nil {
		fail("no manifest.json in %s", abs)
	}

	var id string
	if err := s.Do(func(ex context.Context) error {
		var e error
		id, e = extensions.LoadUnpacked(abs).Do(ex)
		return e
	}); err != nil {
		fail("cannot load the extension: %v", err)
	}
	fmt.Printf("%s✓%s loaded %s\n", ansiGreen, ansiReset, id)
}

func extReload(s *browser.Session, id string) {
	if err := s.Do(func(ex context.Context) error {
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
			return fmt.Errorf("no loaded extension has the id %s", id)
		}
		_, err = extensions.LoadUnpacked(path).Do(ex)
		return err
	}); err != nil {
		fail("cannot reload the extension: %v", err)
	}
	fmt.Printf("%s✓%s reloaded %s\n", ansiGreen, ansiReset, id)
}

func extList(s *browser.Session) {
	var exts []*extensions.ExtensionInfo
	if err := s.Do(func(ex context.Context) error {
		var e error
		exts, e = extensions.GetExtensions().Do(ex)
		return e
	}); err != nil {
		fail("cannot list extensions: %v", err)
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

func extUninstall(s *browser.Session, id string) {
	if err := s.Do(func(ex context.Context) error {
		return extensions.Uninstall(id).Do(ex)
	}); err != nil {
		fail("cannot uninstall the extension: %v", err)
	}
	fmt.Printf("%s✓%s uninstalled %s\n", ansiGreen, ansiReset, id)
}

func extAction(s *browser.Session, id string) {
	// The Extensions domain addresses tab targets, whose IDs differ from the
	// page targets everything else uses; the URL links the two.
	activeURL := ""
	if t, err := s.ActiveTarget(); err == nil && t != nil {
		activeURL = t.URL
	}

	if err := s.Do(func(ex context.Context) error {
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
	}); err != nil {
		fail("cannot trigger the action: %v", err)
	}
	fmt.Printf("%s✓%s triggered action for %s\n", ansiGreen, ansiReset, id)
}
