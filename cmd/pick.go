package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"browser-tools/browser"
	chromedpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

const pickScript = `
(async () => {
	if (!window.__browserToolsPick) {
		window.__browserToolsPick = (message) => new Promise((resolve) => {
			const selections = [];
			const selectedElements = new Set();

			const overlay = document.createElement("div");
			overlay.style.cssText =
				"position:fixed;top:0;left:0;width:100%;height:100%;z-index:2147483647;pointer-events:none";

			const highlight = document.createElement("div");
			highlight.style.cssText =
				"position:absolute;border:2px solid #3b82f6;background:rgba(59,130,246,0.1);transition:all 0.1s";
			overlay.appendChild(highlight);

			const banner = document.createElement("div");
			banner.style.cssText =
				"position:fixed;bottom:20px;left:50%;transform:translateX(-50%);background:#1f2937;color:white;padding:12px 24px;border-radius:8px;font:14px sans-serif;box-shadow:0 4px 12px rgba(0,0,0,0.3);pointer-events:auto;z-index:2147483647";

			const updateBanner = () => {
				banner.textContent = message + " (" + selections.length + " selected, Cmd/Ctrl+Click to add, Enter to finish, ESC to cancel)";
			};
			updateBanner();

			document.body.append(banner, overlay);

			const cleanup = () => {
				document.removeEventListener("mousemove", onMove, true);
				document.removeEventListener("click", onClick, true);
				document.removeEventListener("keydown", onKey, true);
				overlay.remove();
				banner.remove();
				selectedElements.forEach((el) => { el.style.outline = ""; });
			};

			const onMove = (e) => {
				const el = document.elementFromPoint(e.clientX, e.clientY);
				if (!el || overlay.contains(el) || banner.contains(el)) return;
				const r = el.getBoundingClientRect();
				highlight.style.cssText =
					"position:absolute;border:2px solid #3b82f6;background:rgba(59,130,246,0.1);top:" +
					r.top + "px;left:" + r.left + "px;width:" + r.width + "px;height:" + r.height + "px";
			};

			const buildElementInfo = (el) => {
				const parents = [];
				let current = el.parentElement;
				while (current && current !== document.body) {
					const tag = current.tagName.toLowerCase();
					const id = current.id ? "#" + current.id : "";
					const cls = current.className
						? "." + current.className.trim().split(/\s+/).join(".")
						: "";
					parents.push(tag + id + cls);
					current = current.parentElement;
				}
				return {
					tag:     el.tagName.toLowerCase(),
					id:      el.id || null,
					class:   el.className || null,
					text:    (el.textContent || "").trim().slice(0, 200) || null,
					html:    el.outerHTML.slice(0, 500),
					parents: parents.join(" > "),
				};
			};

			const onClick = (e) => {
				if (banner.contains(e.target)) return;
				e.preventDefault();
				e.stopPropagation();
				const el = document.elementFromPoint(e.clientX, e.clientY);
				if (!el || overlay.contains(el) || banner.contains(el)) return;

				if (e.metaKey || e.ctrlKey) {
					if (!selectedElements.has(el)) {
						selectedElements.add(el);
						el.style.outline = "3px solid #10b981";
						selections.push(buildElementInfo(el));
						updateBanner();
					}
				} else {
					cleanup();
					const info = buildElementInfo(el);
					resolve(selections.length > 0 ? selections : info);
				}
			};

			const onKey = (e) => {
				if (e.key === "Escape") {
					e.preventDefault();
					cleanup();
					resolve(null);
				} else if (e.key === "Enter" && selections.length > 0) {
					e.preventDefault();
					cleanup();
					resolve(selections);
				}
			};

			document.addEventListener("mousemove", onMove, true);
			document.addEventListener("click", onClick, true);
			document.addEventListener("keydown", onKey, true);
		});
	}
	return await window.__browserToolsPick(__PICK_MESSAGE__);
})()
`

type elementInfo struct {
	Tag     string  `json:"tag"`
	ID      *string `json:"id"`
	Class   *string `json:"class"`
	Text    *string `json:"text"`
	HTML    string  `json:"html"`
	Parents string  `json:"parents"`
}

func (e elementInfo) print() {
	fmt.Printf("tag: %s\n", e.Tag)
	if e.ID != nil {
		fmt.Printf("id: %s\n", *e.ID)
	}
	if e.Class != nil {
		fmt.Printf("class: %s\n", *e.Class)
	}
	if e.Text != nil {
		fmt.Printf("text: %s\n", *e.Text)
	}
	fmt.Printf("html: %s\n", e.HTML)
	fmt.Printf("parents: %s\n", e.Parents)
}

func PickElement(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("pick-element", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools pick-element <message>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Injects an interactive element picker into the current tab.")
		fmt.Fprintln(os.Stderr, "Click to select one element, Cmd/Ctrl+Click to add multiple,")
		fmt.Fprintln(os.Stderr, "Enter to confirm multi-selection, ESC to cancel.")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	message := strings.Join(fs.Args(), " ")

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(context.Background(), port)

	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	if err := chromedp.Run(tabCtx, browser.ActivateCurrentTab()); err != nil {
		fmt.Fprintf(os.Stderr, "error bringing tab to front: %v\n", err)
		os.Exit(1)
	}

	msgJSON, err := json.Marshal(message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	expr := strings.Replace(pickScript, "__PICK_MESSAGE__", string(msgJSON), 1)

	var raw []byte
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(expr, &raw, func(p *chromedpruntime.EvaluateParams) *chromedpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return
	}

	var single elementInfo
	if err := json.Unmarshal(raw, &single); err == nil && single.Tag != "" {
		single.print()
		return
	}

	var multi []elementInfo
	if err := json.Unmarshal(raw, &multi); err == nil {
		for i, el := range multi {
			if i > 0 {
				fmt.Println()
			}
			el.print()
		}
	}
}
