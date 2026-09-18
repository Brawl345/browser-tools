package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	flag "github.com/spf13/pflag"
)

func SelectDropdown(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	byLabel := fs.Bool("by-label", false, "select the option by its visible label instead of its value")
	byIndex := fs.Bool("by-index", false, "select the option by its 0-based index")
	force := fs.Bool("force", false, "apply even if the element is not visible")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools select [--by-label | --by-index] [--force] <selector> <value>")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *byLabel && *byIndex {
		fail("--by-label and --by-index are mutually exclusive")
	}
	if fs.NArg() < 2 {
		fs.Usage()
		os.Exit(1)
	}

	selector, target := fs.Arg(0), fs.Arg(1)
	if *byIndex {
		if _, err := strconv.Atoi(target); err != nil {
			fail("--by-index needs an integer, got %q", target)
		}
	}

	mode := "value"
	switch {
	case *byLabel:
		mode = "label"
	case *byIndex:
		mode = "index"
	}

	s := MustSession(ctx, variant, port)
	want := StateVisible
	if *force {
		want = StatePresent
	}
	m := Locate(ctx, s, selector, want)

	body := fmt.Sprintf(`const sel = el.tagName.toLowerCase() === "select" ? el : el.closest("select");
		if (!sel) return { ok: false, error: "this is a <" + el.tagName.toLowerCase() + ">, not a <select>" };

		const target = %s, mode = %s;
		const labels = [...sel.options].map(o => o.text.trim());
		let idx = -1;
		if (mode === "index") {
			const i = parseInt(target, 10);
			if (i < 0 || i >= sel.options.length)
				return { ok: false, error: "index " + i + " is out of range (0-" + (sel.options.length - 1) + ")" };
			idx = i;
		} else if (mode === "value") {
			idx = [...sel.options].findIndex(o => o.value === target);
			if (idx === -1) return { ok: false, error: "no option has value " + JSON.stringify(target), options: labels };
		} else {
			idx = labels.indexOf(target);
			if (idx === -1) return { ok: false, error: "no option is labelled " + JSON.stringify(target), options: labels };
		}

		const opt = sel.options[idx];
		const already = sel.selectedIndex === idx;
		if (!already) {
			sel.selectedIndex = idx;
			sel.dispatchEvent(new Event('input',  { bubbles: true }));
			sel.dispatchEvent(new Event('change', { bubbles: true }));
		}
		return { ok: true, already, label: opt.text.trim(), value: opt.value, index: idx };`,
		jsonStr(target), jsonStr(mode))

	settle := SettleAfter(m.Tab)
	var result struct {
		OK      bool     `json:"ok"`
		Already bool     `json:"already"`
		Error   string   `json:"error"`
		Options []string `json:"options"`
		Label   string   `json:"label"`
		Value   string   `json:"value"`
		Index   int      `json:"index"`
	}
	if err := OnHit(m.Tab, body, &result); err != nil {
		fail("%v", err)
	}
	if !result.OK {
		if len(result.Options) > 0 {
			fail("%s — available: %v", result.Error, result.Options)
		}
		fail("%s", result.Error)
	}
	if result.Already {
		fmt.Printf("already selected: %q (value=%q, index=%d)\n", result.Label, result.Value, result.Index)
		return
	}

	settle()
	fmt.Printf("%s✓%s selected: %q (value=%q, index=%d) on %s\n",
		ansiGreen, ansiReset, result.Label, result.Value, result.Index, selector)
}
