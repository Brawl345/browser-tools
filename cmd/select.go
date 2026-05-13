package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func SelectDropdown(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	byLabel := fs.Bool("by-label", false, "select option by visible label instead of value")
	byIndex := fs.Bool("by-index", false, "select option by 0-based index")
	force := fs.Bool("force", false, "apply even if element is not visible")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools select [--by-label | --by-index] [--force] <selector> <value>")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *byLabel && *byIndex {
		fmt.Fprintln(os.Stderr, "error: --by-label and --by-index are mutually exclusive")
		os.Exit(1)
	}
	if fs.NArg() < 2 {
		fs.Usage()
		os.Exit(1)
	}

	selector, target := fs.Arg(0), fs.Arg(1)

	if *byIndex {
		if _, err := strconv.Atoi(target); err != nil {
			fmt.Fprintf(os.Stderr, "error: --by-index requires an integer, got %q\n", target)
			os.Exit(1)
		}
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	if !*force {
		if err := chromedp.Run(tabCtx, chromedp.WaitVisible(selector, chromedp.ByQuery)); err != nil {
			fmt.Fprintf(os.Stderr, "error: element not visible: %s\n", selector)
			fmt.Fprintf(os.Stderr, "hint: use --force if the element is not visible\n")
			os.Exit(1)
		}
	}

	var mode string
	switch {
	case *byLabel:
		mode = "label"
	case *byIndex:
		mode = "index"
	default:
		mode = "value"
	}

	script := fmt.Sprintf(`(function() {
		const sel = document.querySelector(%s);
		if (!sel) return { ok: false, error: "element not found" };
		if (sel.tagName.toLowerCase() !== "select") return { ok: false, error: "element is not a <select>" };

		const target = %s;
		const mode   = %s;
		let idx = -1;

		if (mode === "index") {
			const i = parseInt(target, 10);
			if (i < 0 || i >= sel.options.length)
				return { ok: false, error: "index " + i + " out of range (0–" + (sel.options.length - 1) + ")" };
			idx = i;
		} else if (mode === "value") {
			for (let i = 0; i < sel.options.length; i++) {
				if (sel.options[i].value === target) { idx = i; break; }
			}
			if (idx === -1)
				return { ok: false, error: "no option with value " + JSON.stringify(target) };
		} else {
			for (let i = 0; i < sel.options.length; i++) {
				if (sel.options[i].text.trim() === target) { idx = i; break; }
			}
			if (idx === -1)
				return { ok: false, error: "no option with label " + JSON.stringify(target) };
		}

		const opt = sel.options[idx];
		if (sel.selectedIndex === idx)
			return { ok: true, already: true, label: opt.text.trim(), value: opt.value, index: idx };

		sel.selectedIndex = idx;
		sel.dispatchEvent(new Event('input',  { bubbles: true }));
		sel.dispatchEvent(new Event('change', { bubbles: true }));
		return { ok: true, already: false, label: opt.text.trim(), value: opt.value, index: idx };
	})()`,
		jsonStr(selector),
		jsonStr(target),
		jsonStr(mode),
	)

	var raw []byte
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(script, &raw)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		OK      bool   `json:"ok"`
		Already bool   `json:"already"`
		Error   string `json:"error"`
		Label   string `json:"label"`
		Value   string `json:"value"`
		Index   int    `json:"index"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing result: %v\n", err)
		os.Exit(1)
	}
	if !result.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", result.Error)
		os.Exit(1)
	}
	if result.Already {
		fmt.Printf("already selected: %q (value=%q, index=%d)\n", result.Label, result.Value, result.Index)
		return
	}
	fmt.Printf("%s✓%s selected: %q (value=%q, index=%d) on %s\n",
		ansiGreen, ansiReset, result.Label, result.Value, result.Index, selector)
}
