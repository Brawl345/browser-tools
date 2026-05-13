package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Check(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	uncheck := fs.Bool("uncheck", false, "uncheck the element (checkboxes only)")
	force := fs.Bool("force", false, "set value via JS even if element is not visible")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools check [--uncheck] [--force] <selector>")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	selector := fs.Arg(0)
	wantChecked := !*uncheck

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	// Inspect the element first.
	type elemInfo struct {
		Exists  bool   `json:"exists"`
		Type    string `json:"type"`
		Checked bool   `json:"checked"`
	}
	script := fmt.Sprintf(`(function() {
		const el = document.querySelector(%s);
		if (!el) return { exists: false, type: "", checked: false };
		return { exists: true, type: el.type || el.tagName.toLowerCase(), checked: !!el.checked };
	})()`, jsonStr(selector))

	var raw []byte
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(script, &raw)); err != nil {
		fmt.Fprintf(os.Stderr, "error inspecting element: %v\n", err)
		os.Exit(1)
	}
	var info elemInfo
	if err := json.Unmarshal(raw, &info); err != nil || !info.Exists {
		fmt.Fprintf(os.Stderr, "error: element not found: %s\n", selector)
		os.Exit(1)
	}

	isRadio := info.Type == "radio"

	if isRadio && *uncheck {
		fmt.Fprintln(os.Stderr, "error: cannot uncheck a radio button")
		os.Exit(1)
	}

	if info.Checked == wantChecked {
		state := "checked"
		if !wantChecked {
			state = "unchecked"
		}
		fmt.Printf("already %s: %s\n", state, selector)
		return
	}

	if *force {
		forceScript := fmt.Sprintf(`(function() {
			const el = document.querySelector(%s);
			if (!el) return false;
			el.checked = %v;
			el.dispatchEvent(new Event('input',  { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		})()`, jsonStr(selector), wantChecked)
		var ok bool
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(forceScript, &ok)); err != nil || !ok {
			fmt.Fprintf(os.Stderr, "error: failed to set element: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := chromedp.Run(tabCtx, chromedp.Click(selector, chromedp.ByQuery)); err != nil {
			fmt.Fprintf(os.Stderr, "error clicking element: %v\n", err)
			fmt.Fprintf(os.Stderr, "hint: use --force if the element is not visible\n")
			os.Exit(1)
		}
	}

	action := "checked"
	if !wantChecked {
		action = "unchecked"
	}
	fmt.Printf("%s%s%s %s: %s\n", ansiBold, ansiGreen+"✓"+ansiReset, ansiReset, action, selector)
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
