# AGENTS.md — browser-tools

This folder contains the "browser-tools" skill, which allows AI agents such as Claude to use the Chrome browser to perform various actions. The scripts should always follow the same structure.

`SKILL.md` contains a short description of each tool — this should be concise, as it is loaded into the context window. A full description can be found in `REFERENCE.md`. A short description of each script must be written in `README.md`.

Scripts must not block execution and MUST return a result immediately.

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
Calling the cancel function causes chromedp to call `target.CloseTarget` — **the tab gets closed**.

### Rule

```go
// ✅ correct — discard cancel
tabCtx, _ := chromedp.NewContext(allocCtx, chromedp.WithTargetID(id), ...)

// ❌ wrong — closes the tab
tabCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(id), ...)
defer cancel()
```

Applies to **all** contexts that attach to an existing tab — whether in commands or helper functions like `ActiveTabID`.

The goroutine leaks this causes are irrelevant for a CLI tool (the process exits shortly after anyway).

### Exception

Bootstrap tabs created via `browser.NewTab` may (and should) be cancelled — we created them ourselves and they should be cleaned up after use.
