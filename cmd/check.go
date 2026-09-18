package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/input"
	flag "github.com/spf13/pflag"
)

func Check(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	uncheck := fs.Bool("uncheck", false, "uncheck the element (checkboxes only)")
	force := fs.Bool("force", false, "set the value via JS even if the element is not visible")
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

	s := MustSession(ctx, variant, port)

	want := StateVisible
	if *force {
		want = StatePresent
	}
	m := Locate(ctx, s, selector, want)

	var info struct {
		Type    string `json:"type"`
		Checked bool   `json:"checked"`
	}
	if err := OnHit(m.Tab, `return { type: el.type || el.tagName.toLowerCase(), checked: !!el.checked };`, &info); err != nil {
		fail("%v", err)
	}

	if info.Type == "radio" && *uncheck {
		fail("a radio button cannot be unchecked — check another one in the group instead")
	}

	state := "checked"
	if !wantChecked {
		state = "unchecked"
	}
	if info.Checked == wantChecked {
		fmt.Printf("already %s: %s\n", state, selector)
		return
	}

	settle := SettleAfter(m.Tab)
	if *force {
		body := fmt.Sprintf(`el.checked = %v;
			el.dispatchEvent(new Event('input',  { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
			return true;`, wantChecked)
		if err := OnHit(m.Tab, body, nil); err != nil {
			fail("%v", err)
		}
	} else if err := PointerAt(m.Tab, m.X, m.Y, input.Left, 1); err != nil {
		fail("%v", err)
	}

	settle()
	fmt.Printf("%s✓%s %s: %s%s\n", ansiGreen, ansiReset, state, selector, pointerNote(m))
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
