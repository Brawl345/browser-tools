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

type boxInfo struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type elementInfo struct {
	Tag        string            `json:"tag"`
	ID         *string           `json:"id,omitempty"`
	Class      *string           `json:"class,omitempty"`
	Text       *string           `json:"text,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Box        *boxInfo          `json:"box,omitempty"`
	Visible    bool              `json:"visible"`
	HTML       string            `json:"html,omitempty"`
	Parents    string            `json:"parents,omitempty"`
}

// elementResult wraps the output when a selector matches more than one element.
type elementResult struct {
	Count    int           `json:"count"`
	Note     string        `json:"note,omitempty"`
	Elements []elementInfo `json:"elements"`
}

// elementSampleCap bounds how many matches the in-page script serialises, so a
// broad selector (e.g. "div") can't produce a huge payload.
const elementSampleCap = 10

// elementShowLimit is how many matches are echoed by default on a multi-match.
const elementShowLimit = 3

// buildElementInfoFn is the in-page function that serialises an element into an
// elementInfo. Shared by `element` (resolved via selector) and `pick-element`
// (resolved via click) so both commands return an identical JSON shape.
const buildElementInfoFn = `(el) => {
	const attrs = {};
	for (const a of el.attributes) attrs[a.name] = a.value;
	const r = el.getBoundingClientRect();
	const s = getComputedStyle(el);
	const parents = [];
	let cur = el.parentElement;
	while (cur && cur !== document.body) {
		const ptag = cur.tagName.toLowerCase();
		const pid = cur.id ? "#" + cur.id : "";
		const pcls = (typeof cur.className === "string" && cur.className.trim())
			? "." + cur.className.trim().split(/\s+/).join(".") : "";
		parents.push(ptag + pid + pcls);
		cur = cur.parentElement;
	}
	return {
		tag:        el.tagName.toLowerCase(),
		id:         el.id || null,
		class:      (typeof el.className === "string" && el.className.trim()) ? el.className : null,
		text:       (el.textContent || "").trim().slice(0, 200) || null,
		attributes: attrs,
		box:        { x: r.x, y: r.y, width: r.width, height: r.height },
		visible:    r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none",
		html:       el.outerHTML.slice(0, 500),
		parents:    parents.join(" > "),
	};
}`

func Element(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("element", flag.ExitOnError)
	attr := fs.String("attr", "", "print only this attribute's value as plain text")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools element [--attr <name>] <selector>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Reads tag, id, class, text, attributes, bounding box and visibility")
		fmt.Fprintln(os.Stderr, "of the first element matching the CSS selector. Prints JSON.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}
	sel := fs.Arg(0)

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	script := fmt.Sprintf(`(function(){
	const els = document.querySelectorAll(%s);
	if (!els.length) return null;
	const out = [];
	for (let i = 0; i < els.length && i < %d; i++) out.push((%s)(els[i]));
	return { count: els.length, elements: out };
})()`, jsonStr(sel), elementSampleCap, buildElementInfoFn)

	var raw []byte
	if err := chromedp.Run(tabCtx,
		chromedp.WaitReady(sel, chromedp.ByQuery),
		chromedp.Evaluate(script, &raw),
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(raw) == 0 || string(raw) == "null" {
		fmt.Fprintf(os.Stderr, "error: element not found: %s\n", sel)
		os.Exit(1)
	}

	var res elementResult
	if err := json.Unmarshal(raw, &res); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *attr != "" {
		printAttr(res, *attr, sel)
		return
	}

	shown := res.Elements
	if len(shown) > elementShowLimit {
		shown = shown[:elementShowLimit]
	}
	out := elementResult{Count: res.Count, Elements: shown}
	if res.Count > 1 {
		out.Note = fmt.Sprintf("%d elements match %q; showing the first %d. Refine the selector (e.g. a parent prefix or :nth-of-type) to target a single element.",
			res.Count, sel, len(shown))
	}
	printJSON(out)
}

func printAttr(res elementResult, attr, sel string) {
	printed := 0
	for _, el := range res.Elements {
		if v, ok := el.Attributes[attr]; ok {
			fmt.Println(v)
			printed++
		}
	}
	if printed == 0 {
		fmt.Fprintf(os.Stderr, "error: attribute %q not present on any of %d match(es) for %s\n", attr, res.Count, sel)
		os.Exit(1)
	}
	if res.Count > len(res.Elements) {
		fmt.Fprintf(os.Stderr, "note: %d elements match; printed the attribute for the first %d only — refine the selector for the rest\n",
			res.Count, len(res.Elements))
	}
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}
