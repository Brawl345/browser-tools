package cmd

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

// elementDetailCap is how many matches are described in full, elementListCap how
// many are listed in the short form, and elementAttrCap how many attribute
// values are printed.
const (
	elementDetailCap = 1
	elementListCap   = 10
	elementAttrCap   = 100
)

// buildElementInfoFn serialises one element. Shared with pick-element so both
// commands return the same shape. `full` adds the parts that only make sense
// for a single element.
const buildElementInfoFn = `(el, full) => {
	const r = el.getBoundingClientRect();
	const s = getComputedStyle(el);
	const out = {
		tag:     el.tagName.toLowerCase(),
		id:      el.id || undefined,
		class:   (typeof el.className === "string" && el.className.trim()) ? el.className : undefined,
		text:    (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 200) || undefined,
		box:     { x: Math.round(r.x), y: Math.round(r.y), width: Math.round(r.width), height: Math.round(r.height) },
		visible: r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none" && s.opacity !== "0",
	};
	if ("value" in el && el.value !== undefined && el.value !== "") out.value = String(el.value).slice(0, 200);
	if (el.checked !== undefined) out.checked = el.checked;
	if (el.disabled) out.disabled = true;
	if (!full) return out;

	const attrs = {};
	for (const a of el.attributes) attrs[a.name] = a.value;
	out.attributes = attrs;

	const style = { display: s.display, cursor: s.cursor, color: s.color, backgroundColor: s.backgroundColor, fontSize: s.fontSize };
	const extra = { position: "static", zIndex: "auto", opacity: "1", visibility: "visible", pointerEvents: "auto", overflow: "visible" };
	for (const k in extra) if (s[k] !== extra[k]) style[k] = s[k];
	out.style = style;

	const parents = [];
	let cur = el.parentElement;
	while (cur && cur !== document.body && parents.length < 8) {
		parents.push(cur.tagName.toLowerCase() +
			(cur.id ? "#" + cur.id : "") +
			(typeof cur.className === "string" && cur.className.trim() ? "." + cur.className.trim().split(/\s+/).join(".") : ""));
		cur = cur.parentElement;
	}
	out.parents = parents.join(" > ") || undefined;
	out.html = el.outerHTML.slice(0, 500);
	return out;
}`

func Element(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("element", flag.ExitOnError)
	attr := fs.String("attr", "", "print one value per match as plain text: an attribute name, or text / html / value")
	limit := fs.Int("limit", 0, fmt.Sprintf("print at most this many matches (default %d, with --attr %d)", elementListCap, elementAttrCap))
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools element [--attr <name>] [--limit <n>] <selector>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Reads tag, text, value, attributes, box, computed style and visibility")
		fmt.Fprintln(os.Stderr, "as JSON. With several matches it prints a short list of all of them.")
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
	if *limit < 0 {
		fail("--limit must be at least 1")
	}

	s := MustSession(ctx, variant, port)
	m := Locate(ctx, s, sel, StatePresent)

	if *attr != "" {
		printAttr(m, *attr, sel, cmp.Or(*limit, elementAttrCap))
		return
	}

	full := "false"
	n := cmp.Or(*limit, elementListCap)
	if m.Count <= elementDetailCap {
		full = "true"
		n = elementDetailCap
	}
	body := fmt.Sprintf(`const hits = window.__btHits || [];
		return hits.slice(0, %d).map(e => (%s)(e, %s));`, n, buildElementInfoFn, full)

	var elements []json.RawMessage
	if err := OnHit(m.Tab, body, &elements); err != nil {
		fail("%v", err)
	}

	out := map[string]any{"count": m.Count, "elements": elements}
	if m.Count > len(elements) {
		out["note"] = fmt.Sprintf("showing %d of %d matches; narrow the selector, raise --limit, or use --attr to list one value per match", len(elements), m.Count)
	}
	printJSON(out)
}

// attrBody returns the JS expression that reads the wanted value off `e`.
func attrBody(attr string) string {
	switch attr {
	case "text":
		return `(e.textContent || "").replace(/[ \t]+/g, " ").replace(/\n{3,}/g, "\n\n").trim()`
	case "html":
		return `e.outerHTML`
	case "value":
		return `("value" in e ? String(e.value) : (e.getAttribute("value") || ""))`
	default:
		// Fall back to the DOM property, so live state such as `checked` or
		// `disabled` is readable even though it is not an attribute.
		return fmt.Sprintf(`(e.hasAttribute(%[1]s) ? e.getAttribute(%[1]s) : (%[1]s in e ? String(e[%[1]s]) : ""))`, jsonStr(attr))
	}
}

func printAttr(m *Match, attr, sel string, limit int) {
	body := fmt.Sprintf(`const hits = window.__btHits || [];
		return hits.slice(0, %d).map(e => %s);`, limit, attrBody(attr))

	var values []string
	if err := OnHit(m.Tab, body, &values); err != nil {
		fail("%v", err)
	}

	printed := 0
	for _, v := range values {
		if v == "" {
			continue
		}
		fmt.Println(v)
		printed++
	}
	if printed == 0 {
		fail("no match for %s carries %q — `browser-tools element %s` shows what is there", sel, attr, sel)
	}
	if m.Count > len(values) {
		fmt.Fprintf(os.Stderr, "note: printed %d of %d matches, raise --limit for more\n", len(values), m.Count)
	}
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail("%v", err)
	}
	fmt.Println(string(b))
}
