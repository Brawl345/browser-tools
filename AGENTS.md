# AGENTS.md: browser-tools

This folder contains the "browser-tools" skill, which allows AI agents such as Claude to use the Chrome browser to perform various actions. The scripts should always follow the same structure.

`SKILL.md` contains a short description of each tool, which has to stay concise because it is loaded into the context window. A full description can be found in `REFERENCE.md`. A short description of each script must be written in `README.md`.

Every command has to come back with a result inside `--timeout`. Waiting for an element or a page load is part of that; waiting without a bound is a bug.

## Architecture

`browser.Session` owns the connection. It allocates the chromedp browser by hand, so `Target.getTargets` and the Extensions domain are reachable without opening a tab, and it caches one attached context per tab.

`cmd/locate.go` is the single selector engine. Every command that takes a selector goes through `Locate`, which polls one in-page script handling CSS, XPath and `text=`, re-resolves the active tab while it waits, and renders the failure report. Commands act on what it returns: `Match.X/Y` for pointer events, `window.__btHit` for JS, `Match.Path` where chromedp needs a CSS selector.

`cmd/settle.go` holds the waiting that follows an action: `SettleAfter` around anything that changes the page, `WaitLoaded` after a navigation.

## Build

```bash
go build -o scripts/browser-tools .
```

## References

* [Skills documentation](https://docs.claude.com/en/docs/claude-code/skills)
* [chromerdp docs](https://pkg.go.dev/github.com/chromedp/chromedp)

## Common pitfalls

## Do not query the JSON API manually

Never query the Chrome RDP API manually.

## NEVER call cancel functions on existing tabs

`chromedp.NewContext` with `chromedp.WithTargetID` on an **existing** tab sets `c.first = true` internally.
Calling the cancel function makes chromedp call `target.CloseTarget`, which closes the tab.

### Rule

```go
// ✅ correct: discard cancel
tabCtx, _ := chromedp.NewContext(allocCtx, chromedp.WithTargetID(id), ...)

// ❌ wrong: closes the tab
tabCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(id), ...)
defer cancel()
```

This applies to every context that attaches to an existing tab: `browser.AttachTab` and everything built on it.

The goroutine leaks this causes are irrelevant for a CLI tool (the process exits shortly after anyway).

### Exception

Tabs created via `browser.NewTab` may be cancelled, because we created them ourselves.

## Bound every round trip to a tab

Because those contexts carry no cancellation, `chromedp.Run` against a tab that has closed itself never returns. Pages do close their own tab: a Tampermonkey install dialog vanishes the moment the button is clicked. Use `cmd.RunBounded` instead of `chromedp.Run` for anything aimed at a tab.
