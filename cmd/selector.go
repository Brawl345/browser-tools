package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// IsXPath reports whether sel is an XPath expression. A CSS selector never
// starts with a slash, and `./` cannot begin a class name.
func IsXPath(sel string) bool {
	s := strings.TrimSpace(sel)
	return strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "(/") ||
		strings.HasPrefix(s, "(./")
}

// cssPathFn is the in-page function that writes an element down as a CSS path
// of :nth-of-type steps, so an XPath result can be handed to the rest of a
// command as a plain CSS selector.
const cssPathFn = `(el) => {
	const parts = [];
	while (el && el.nodeType === 1 && el !== document.documentElement) {
		const parent = el.parentElement;
		if (!parent) break;
		let i = 1;
		for (let s = el.previousElementSibling; s; s = s.previousElementSibling) {
			if (s.localName === el.localName) i++;
		}
		parts.unshift(CSS.escape(el.localName) + ":nth-of-type(" + i + ")");
		el = parent;
	}
	parts.unshift("html");
	return parts.join(" > ");
}`

// xpathPathsScript evaluates the expression and returns a CSS path per matched
// element.
func xpathPathsScript(xpath string) string {
	return fmt.Sprintf(`(function(){
		let r;
		try {
			r = document.evaluate(%s, document, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
		} catch (e) {
			return { paths: [], other: 0, error: String((e && e.message) || e) };
		}
		const paths = [];
		let other = 0;
		for (let i = 0; i < r.snapshotLength; i++) {
			const n = r.snapshotItem(i);
			if (n && n.nodeType === 1) paths.push((%s)(n)); else other++;
		}
		return { paths: paths, other: other, error: "" };
	})()`, jsonStr(xpath), cssPathFn)
}

// ResolveSelector turns an XPath expression into a CSS selector for the matched
// elements and passes a CSS selector through unchanged.
func ResolveSelector(ctx context.Context, sel string) (string, error) {
	if !IsXPath(sel) {
		return sel, nil
	}

	var raw []byte
	if err := chromedp.Run(ctx, chromedp.Evaluate(xpathPathsScript(sel), &raw)); err != nil {
		return "", err
	}
	var res struct {
		Paths []string `json:"paths"`
		Other int      `json:"other"`
		Error string   `json:"error"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("invalid XPath %q: %s", sel, res.Error)
	}
	if len(res.Paths) == 0 {
		if res.Other > 0 {
			return "", fmt.Errorf("XPath %q matched %d node(s) that are not elements — select the element itself, without a trailing /text() or /@attr", sel, res.Other)
		}
		return "", fmt.Errorf("no element matches XPath: %s", sel)
	}
	return strings.Join(res.Paths, ", "), nil
}

// MustResolveSelector resolves the selector or ends the process with an error.
func MustResolveSelector(ctx context.Context, sel string) string {
	css, err := ResolveSelector(ctx, sel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return css
}

// xpathStateScript checks the state of the first element an XPath matches.
func xpathStateScript(xpath string) string {
	return fmt.Sprintf(`(function(){
		let r;
		try {
			r = document.evaluate(%s, document, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
		} catch (e) {
			return { found: false, visible: false, error: String((e && e.message) || e) };
		}
		for (let i = 0; i < r.snapshotLength; i++) {
			const el = r.snapshotItem(i);
			if (!el || el.nodeType !== 1) continue;
			const b = el.getBoundingClientRect();
			const s = getComputedStyle(el);
			return {
				found: true,
				visible: b.width > 0 && b.height > 0 && s.visibility !== "hidden" && s.display !== "none",
				error: "",
			};
		}
		return { found: false, visible: false, error: "" };
	})()`, jsonStr(xpath))
}

// WaitXPath polls the page until the element an XPath matches reaches the wanted
// state ("visible", "hidden", "present" or "absent"). The deadline of ctx bounds
// the wait; the context itself is never cancelled, since that would close the
// tab.
func WaitXPath(ctx, tabCtx context.Context, xpath, state string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}

	for {
		var raw []byte
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(xpathStateScript(xpath), &raw)); err != nil {
			return err
		}
		var res struct {
			Found   bool   `json:"found"`
			Visible bool   `json:"visible"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if res.Error != "" {
			return fmt.Errorf("invalid XPath %q: %s", xpath, res.Error)
		}

		var reached bool
		switch state {
		case "hidden":
			reached = res.Found && !res.Visible
		case "present":
			reached = res.Found
		case "absent":
			reached = !res.Found
		default:
			reached = res.Found && res.Visible
		}
		if reached {
			return nil
		}
		if time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		time.Sleep(100 * time.Millisecond)
	}
}
