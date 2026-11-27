package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"browser-tools/browser"

	"github.com/chromedp/chromedp"
)

// Selector states a Locate call can wait for.
const (
	StateVisible = "visible"
	StatePresent = "present"
	StateHidden  = "hidden"
	StateAbsent  = "absent"
)

// pollInterval is how often the page is asked again while waiting.
const pollInterval = 120 * time.Millisecond

// retargetInterval is how often the active tab is looked up again during a
// wait, so a dialog opening in a new tab is picked up mid-wait.
const retargetInterval = 400 * time.Millisecond

// Match describes the element a selector resolved to.
type Match struct {
	Count   int     `json:"n"`
	Path    string  `json:"path"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Visible bool    `json:"visible"`
	Covered string  `json:"covered"`

	// Tab is the tab the element was found in.
	Tab context.Context `json:"-"`
}

// SelectorKind classifies a selector string.
func SelectorKind(sel string) string {
	s := strings.TrimSpace(sel)
	switch {
	case strings.HasPrefix(s, "text="):
		return "text"
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, "./"),
		strings.HasPrefix(s, "(/"), strings.HasPrefix(s, "(./"):
		return "xpath"
	default:
		return "css"
	}
}

// findScript is the single in-page resolver behind every selector. It handles
// CSS, XPath and text= selectors, looks inside open shadow roots when the light
// DOM has no match, and parks the winner on window.__btHit so follow-up scripts
// can act on it without a second lookup.
const findScript = `(function(sel, kind, want){
	// Coerced, because properties such as title are an SVGTitleElement on SVG
	// nodes rather than a string.
	const norm = s => (s == null ? "" : String(s)).replace(/\s+/g, " ").trim();

	function shadowRoots() {
		const out = [];
		const walk = root => {
			for (const el of root.querySelectorAll("*")) {
				if (el.shadowRoot) { out.push(el.shadowRoot); walk(el.shadowRoot); }
			}
		};
		walk(document);
		return out;
	}

	function byCSS(s) {
		let hits = [];
		try { hits = [...document.querySelectorAll(s)]; }
		catch (e) { return { error: "invalid CSS selector: " + e.message }; }
		if (!hits.length) {
			for (const r of shadowRoots()) hits.push(...r.querySelectorAll(s));
		}
		return { hits };
	}

	function byXPath(s) {
		let r;
		try { r = document.evaluate(s, document, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null); }
		catch (e) { return { error: "invalid XPath: " + e.message }; }
		const hits = [];
		let other = 0;
		for (let i = 0; i < r.snapshotLength; i++) {
			const n = r.snapshotItem(i);
			if (n && n.nodeType === 1) hits.push(n); else other++;
		}
		if (!hits.length && other) {
			return { error: "XPath matched " + other + " node(s) that are not elements — drop a trailing /text() or /@attr" };
		}
		return { hits };
	}

	const SKIP = { SCRIPT: 1, STYLE: 1, HEAD: 1, TITLE: 1, NOSCRIPT: 1, META: 1, LINK: 1, TEMPLATE: 1 };

	// Text of every element in one bottom-up pass, with script and style
	// contents left out so a wrapper does not inherit source code as its label.
	// Each entry is cut off at cap: an element carrying more text than that is
	// an article, not a label, and keeping whole page texts around would make
	// the pass cost time quadratic in the size of the document.
	function textMap(root, cap) {
		const m = new Map();
		const els = [...root.querySelectorAll("*")];
		for (let i = els.length - 1; i >= 0; i--) {
			const el = els[i];
			if (SKIP[el.tagName]) { m.set(el, ""); continue; }
			let s = "";
			for (const n of el.childNodes) {
				if (n.nodeType === 3) s += n.nodeValue;
				else if (n.nodeType === 1) s += (m.get(n) || "");
				if (s.length > cap) break;
			}
			m.set(el, s.slice(0, cap));
		}
		return m;
	}

	function labels(el, text) {
		const t = el.tagName;
		const out = [el.getAttribute("aria-label"), el.title];
		if (t === "INPUT" || t === "TEXTAREA" || t === "SELECT") {
			out.push(el.value, el.placeholder);
		} else if (t === "IMG") {
			out.push(el.alt);
		} else {
			out.push(text, el.getAttribute("value"));
		}
		return out.map(norm).filter(Boolean);
	}

	// labelCap is the longest text an element may carry and still count as a
	// label for it.
	function byText(needle) {
		const want = norm(needle).toLowerCase();
		const labelCap = Math.max(400, want.length * 2);
		const exact = [], loose = [];
		for (const r of [document, ...shadowRoots()]) {
			const texts = textMap(r, labelCap);
			for (const el of r.querySelectorAll("*")) {
				if (SKIP[el.tagName]) continue;
				const ls = labels(el, texts.get(el) || "");
				if (ls.some(l => l.toLowerCase() === want)) exact.push(el);
				else if (ls.some(l => l.toLowerCase().includes(want))) loose.push(el);
			}
		}
		let hits = exact.length ? exact : loose;
		// Keep the innermost carrier of the text, so a wrapper div does not win
		// over the button inside it.
		hits = hits.slice(0, 300);
		hits = hits.filter(el => !hits.some(o => o !== el && el.contains(o)));
		// A visible match always beats a hidden duplicate.
		hits.sort((a, b) => (visible(b) ? 1 : 0) - (visible(a) ? 1 : 0));
		return { hits };
	}

	// Visibility follows what a user can interact with: laid out, not hidden by
	// the box model. Opacity is left out because fade-in overlays are clickable
	// while they animate.
	function visible(el) {
		const r = el.getBoundingClientRect();
		const s = getComputedStyle(el);
		return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none";
	}

	function cssPath(el) {
		if (el.getRootNode() !== document) return "";
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
	}

	const res = kind === "xpath" ? byXPath(sel) : kind === "text" ? byText(sel.slice(5)) : byCSS(sel);
	if (res.error) return { error: res.error };

	const hits = res.hits;
	window.__btHits = hits;
	if (!hits.length) {
		window.__btHit = null;
		return { n: 0, reached: want === "absent" };
	}

	const el = hits[0];
	window.__btHit = el;
	const vis = visible(el);
	const reached = want === "visible" ? vis : want === "hidden" ? !vis : want === "present";

	// Bring it into view even while still waiting: many players and lazy
	// sections only finish rendering once they are near the viewport, and the
	// box below is measured in viewport coordinates.
	if (want !== "hidden") {
		try { el.scrollIntoView({ block: "center", inline: "center", behavior: "instant" }); } catch (e) {}
	}
	const r = el.getBoundingClientRect();
	const x = r.left + r.width / 2, y = r.top + r.height / 2;

	// Something drawn on top swallows a click aimed at this point, so the
	// caller can say whose element actually receives it.
	let covered = "";
	if (vis && x > 0 && y > 0 && x < innerWidth && y < innerHeight) {
		const top = document.elementFromPoint(x, y);
		if (top && top !== el && !el.contains(top) && !top.contains(el)) {
			covered = top.tagName.toLowerCase() +
				(top.id ? "#" + top.id : "") +
				(typeof top.className === "string" && top.className.trim() ? "." + top.className.trim().split(/\s+/).join(".") : "");
		}
	}

	return { n: hits.length, reached, visible: vis, path: cssPath(el), x, y, covered };
})`

// diagnoseScript explains why a selector did not resolve: how far a CSS
// selector got, which texts are actually on the page, or what covers the
// element that was found.
const diagnoseScript = `(function(sel, kind){
	const norm = s => (s == null ? "" : String(s)).replace(/\s+/g, " ").trim();
	const el = window.__btHit;

	const where = { url: location.href, title: document.title };

	if (el) {
		const r = el.getBoundingClientRect();
		const s = getComputedStyle(el);
		const reasons = [];
		if (s.display === "none") reasons.push("display:none");
		if (s.visibility === "hidden") reasons.push("visibility:hidden");
		if (s.opacity === "0") reasons.push("opacity:0");
		if (r.width === 0 || r.height === 0) reasons.push("box is " + Math.round(r.width) + "x" + Math.round(r.height));
		let cover = "";
		if (r.width > 0 && r.height > 0) {
			const cx = Math.min(Math.max(r.left + r.width / 2, 1), innerWidth - 1);
			const cy = Math.min(Math.max(r.top + r.height / 2, 1), innerHeight - 1);
			const top = document.elementFromPoint(cx, cy);
			if (top && top !== el && !el.contains(top) && !top.contains(el)) {
				cover = top.tagName.toLowerCase() +
					(top.id ? "#" + top.id : "") +
					(typeof top.className === "string" && top.className.trim() ? "." + top.className.trim().split(/\s+/).join(".") : "");
			}
		}
		return { ...where, found: true, reasons, cover, tag: el.tagName.toLowerCase() };
	}

	if (kind === "css") {
		// Strip the selector from the right until something matches, so the
		// report names the part that is wrong.
		const parts = sel.trim().split(/\s+(?![^\[]*\])/);
		for (let i = parts.length - 1; i > 0; i--) {
			const prefix = parts.slice(0, i).join(" ");
			let n = 0;
			try { n = document.querySelectorAll(prefix).length; } catch (e) { continue; }
			if (n) return { ...where, found: false, closest: prefix, closestCount: n };
		}
		return { ...where, found: false };
	}

	if (kind === "text") {
		const want = norm(sel.slice(5)).toLowerCase();
		const words = want.split(" ").filter(w => w.length > 2);
		const seen = new Set();
		const out = [];
		for (const e of document.querySelectorAll("button, a, input, [role=button], summary, label, h1, h2, h3")) {
			const t = norm(e.tagName === "INPUT" ? (e.value || e.placeholder) : e.textContent);
			if (!t || t.length > 60 || seen.has(t)) continue;
			seen.add(t);
			const hit = words.some(w => t.toLowerCase().includes(w));
			out.push({ t, hit });
		}
		out.sort((a, b) => (b.hit ? 1 : 0) - (a.hit ? 1 : 0));
		return { ...where, found: false, texts: out.slice(0, 8).map(o => o.t) };
	}

	return { ...where, found: false };
})`

type findResult struct {
	N       int     `json:"n"`
	Reached bool    `json:"reached"`
	Visible bool    `json:"visible"`
	Covered string  `json:"covered"`
	Path    string  `json:"path"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Error   string  `json:"error"`
}

type diagResult struct {
	URL          string   `json:"url"`
	Title        string   `json:"title"`
	Found        bool     `json:"found"`
	Reasons      []string `json:"reasons"`
	Cover        string   `json:"cover"`
	Tag          string   `json:"tag"`
	Closest      string   `json:"closest"`
	ClosestCount int      `json:"closestCount"`
	Texts        []string `json:"texts"`
}

// Locate polls until the selector reaches the wanted state and returns the
// match. The active tab is looked up again while polling, so an element that
// only appears in a tab opened meanwhile is still found. On failure it prints a
// report naming the tab, the state of the element and what to try instead, then
// ends the process.
func Locate(ctx context.Context, s *browser.Session, sel, want string) *Match {
	m, err := TryLocate(ctx, s, sel, want)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	return m
}

// TryLocate is Locate without the exit, for callers that recover from a miss.
func TryLocate(ctx context.Context, s *browser.Session, sel, want string) (*Match, error) {
	kind := SelectorKind(sel)
	script := fmt.Sprintf("(%s)(%s, %s, %s)", findScript, jsonStr(sel), jsonStr(kind), jsonStr(want))

	deadline := budgetDeadline(ctx)
	start := time.Now()
	var (
		tabCtx   context.Context
		lastLook time.Time
		last     findResult
		lastErr  error
		scan     scanner
	)

	for {
		if tabCtx == nil || time.Since(lastLook) > retargetInterval {
			if c, err := s.ActiveTab(); err == nil && c != nil {
				tabCtx = c
			}
			lastLook = time.Now()
		}
		if tabCtx == nil {
			return nil, fmt.Errorf("error: no browser tab is open\n")
		}

		var raw []byte
		err := runWithin(tabCtx, time.Until(deadline), chromedp.Evaluate(script, &raw))
		if err != nil {
			lastErr = err
			// The tab may have gone away under us, so look again at which one
			// is in front before the next attempt.
			lastLook = time.Time{}
		} else {
			lastErr = nil
			last = findResult{}
			if json.Unmarshal(raw, &last) == nil {
				if last.Error != "" {
					return nil, fmt.Errorf("error: %s: %s\n", sel, last.Error)
				}
				if last.Reached {
					return &Match{
						Count: last.N, Path: last.Path, X: last.X, Y: last.Y,
						Visible: last.Visible, Covered: last.Covered, Tab: tabCtx,
					}, nil
				}
			}
		}

		if !time.Now().Before(deadline) {
			if lastErr != nil {
				return nil, fmt.Errorf("error: %s\n  while looking for %s\n  check `browser-tools tab` to see which tabs are left\n", explainTabError(lastErr), sel)
			}
			scan.restore(tabCtx)
			return nil, locateError(tabCtx, sel, kind, want, last, s, scan.used)
		}
		if err == nil && last.N == 0 && want != StateAbsent && time.Since(start) > scanAfter {
			scan.step(tabCtx)
		}
		time.Sleep(pollInterval)
	}
}

// scanAfter is how long a selector may stay unmatched before the page is
// scrolled through to wake up lazily rendered content.
const scanAfter = 700 * time.Millisecond

// scanSteps is how many positions the page is walked through, top to bottom.
const scanSteps = 6

// scanner scrolls the page in stages while a selector stays unmatched, because
// plenty of sites only build a section once it comes near the viewport. The
// original position is put back when the search fails.
type scanner struct {
	at       int
	used     bool
	origin   float64
	restored bool
}

func (s *scanner) step(tabCtx context.Context) {
	if s.at > scanSteps {
		return
	}
	if !s.used {
		_ = RunBounded(tabCtx, chromedp.Evaluate(`window.scrollY`, &s.origin))
		s.used = true
	}
	frac := float64(s.at) / scanSteps
	s.at++
	_ = RunBounded(tabCtx, chromedp.Evaluate(fmt.Sprintf(
		`window.scrollTo({top: (document.documentElement.scrollHeight - innerHeight) * %g, behavior: "instant"})`, frac), nil))
}

func (s *scanner) restore(tabCtx context.Context) {
	if !s.used || s.restored {
		return
	}
	s.restored = true
	_ = RunBounded(tabCtx, chromedp.Evaluate(fmt.Sprintf(
		`window.scrollTo({top: %g, behavior: "instant"})`, s.origin), nil))
}

// budgetDeadline leaves the watchdog a margin, so a command always reports its
// own precise error before the global timeout kills the process.
func budgetDeadline(ctx context.Context) time.Time {
	d, ok := ctx.Deadline()
	if !ok {
		return time.Now().Add(10 * time.Second)
	}
	return d.Add(-600 * time.Millisecond)
}

// locateError renders the failure report.
func locateError(tabCtx context.Context, sel, kind, want string, last findResult, s *browser.Session, scanned bool) error {
	var b strings.Builder

	var diag diagResult
	var raw []byte
	if err := RunBounded(tabCtx, chromedp.Evaluate(
		fmt.Sprintf("(%s)(%s, %s)", diagnoseScript, jsonStr(sel), jsonStr(kind)), &raw)); err == nil {
		_ = json.Unmarshal(raw, &diag)
	}

	switch {
	case last.N == 0:
		fmt.Fprintf(&b, "error: nothing matches %s\n", sel)
	case want == StateVisible:
		fmt.Fprintf(&b, "error: %s matches %d element(s) but none is visible\n", sel, last.N)
	default:
		fmt.Fprintf(&b, "error: %s did not become %s (%d match(es))\n", sel, want, last.N)
	}

	fmt.Fprintf(&b, "  tab: %s — %s\n", firstNonEmpty(diag.Title, "(no title)"), firstNonEmpty(diag.URL, "(unknown)"))

	if diag.Found {
		if len(diag.Reasons) > 0 {
			fmt.Fprintf(&b, "  the <%s> is there but %s\n", diag.Tag, strings.Join(diag.Reasons, ", "))
		}
		if diag.Cover != "" {
			fmt.Fprintf(&b, "  covered by %s — click that instead, or pass --force\n", diag.Cover)
		}
	}
	if diag.Closest != "" {
		fmt.Fprintf(&b, "  closest hit: %q matches %d element(s)\n", diag.Closest, diag.ClosestCount)
	}
	if len(diag.Texts) > 0 {
		fmt.Fprintf(&b, "  texts on the page: %s\n", strings.Join(quoteAll(diag.Texts), ", "))
	}

	if scanned {
		fmt.Fprintf(&b, "  the whole page was scrolled through, so lazily rendered parts were given a chance\n")
	}
	if tabs, err := s.Tabs(); err == nil && len(tabs) > 1 {
		fmt.Fprintf(&b, "  %d tabs are open — check `browser-tools tab` if this is the wrong one\n", len(tabs))
	}
	return fmt.Errorf("%s", b.String())
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
