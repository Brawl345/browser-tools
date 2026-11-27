---
name: browser-tools
description: Interact with a web browser. Can start a browser, connect to it, evaluate JavaScript, make screenshots, read console logs and let the user select DOM elements. Use when interacting with unknown websites (e.g. scraping or Userscripts) or debugging browser-stuff.
compatibility: Requires Chrome
---

# Browser Tools

A compiled Go binary that drives Chrome over the DevTools Protocol. Run it with its full path:

```bash
{basedir}/scripts/browser-tools <command> [options]
```

Every command waits by itself. It polls until the element is there, the page has gone quiet and animations have ended, so adding `sleep` only makes things slower. Commands act on the current tab, the one last opened, navigated or activated. A popup, a link opening a new tab, or an extension dialog becomes current by itself, even mid-wait, and closing it returns to the tab before. When a selector does not resolve, the error names the tab and says whether the element was missing, invisible or covered.

## Selectors

Anywhere a selector is taken, three kinds work. Text is usually the shortest:

```bash
./scripts/browser-tools mouse click "text=Sign in"      # visible text, button value, aria-label, placeholder, alt
./scripts/browser-tools mouse click "#submit"           # CSS, reaches into open shadow roots
./scripts/browser-tools mouse click "//tr[3]//a"        # XPath (starts with / or ./)
```

`text=` matches case-insensitively, prefers an exact label over a partial one, and picks the innermost element carrying the text. On a page that comes in several languages the label changes with the locale, so a CSS selector holds up better there.

## Do this, not JavaScript

`evaluate-js` is the last resort. These cover almost everything agents write scripts for:

| Instead of | Use |
|---|---|
| `el.click()`, `dispatchEvent` | `mouse click <sel>` (`--force` for hidden elements) |
| `el.textContent`, `outerHTML` | `element <sel> --attr text` / `--attr html` |
| `getBoundingClientRect`, `getComputedStyle` | `element <sel>`, whose JSON has `box` and `style` |
| `querySelectorAll(...).length` | `element <sel>`, whose JSON has `count` |
| `el.value = x`, `el.checked = true` | `fill <sel> <text>`, `check <sel>` |
| `scrollIntoView`, `window.scrollTo` | `scroll <sel>` / `scroll --y <n>` |
| `location.href = x`, `location.reload()` | `navigate <url>`, `reload` |
| `localStorage`, `document.cookie` | `dom-storage`, `cookie` |
| `setTimeout` / polling loops | `wait <sel>` |

## Pages

```bash
./scripts/browser-tools navigate https://example.com     # waits for the load event
./scripts/browser-tools navigate https://example.com --new-tab
./scripts/browser-tools reload                           # --hard bypasses the cache
./scripts/browser-tools scroll "#section"                # or --x/--y/--top/--bottom
./scripts/browser-tools resize 1280 720                   # stays until --reset; --mobile --dpr 3 393 852 for a phone
./scripts/browser-tools resize --color-scheme dark        # dark/light mode, no restart needed
./scripts/browser-tools tab                               # in opening order, ▶ = current; --activate/--close/--refresh <n>
```

## Reading

```bash
./scripts/browser-tools element "button#submit"           # JSON: count, text, value, attributes, box, style, visible
./scripts/browser-tools element "a.download" --attr href  # one line per match: any attribute, or text / html / value
./scripts/browser-tools html --filter "data-id=\"\d+\"" --lines 10
./scripts/browser-tools console --errors-only             # includes messages logged before it ran
./scripts/browser-tools cookie                            # --all for every origin
./scripts/browser-tools dom-storage                       # --local / --session
```

With one match `element` prints everything about it; with several it prints a short entry per match.

## Acting

```bash
./scripts/browser-tools mouse click "text=Sign in"        # also dblclick, hover, right-click
./scripts/browser-tools mouse drag ".card" --to ".column-2"
./scripts/browser-tools fill "input#user" "john" --clear
./scripts/browser-tools check "input#terms"               # --uncheck to clear it
./scripts/browser-tools select "select#country" "US"      # --by-label / --by-index
./scripts/browser-tools key Enter --selector "input#search"
./scripts/browser-tools upload "input[type=file]" a.jpg b.png
./scripts/browser-tools --timeout 60s download "a.report" --output ~/report.pdf
```

Add `--force` to `mouse`, `check` or `select` to act through JavaScript on an element that is not visible.

## Waiting

Only needed for a state the acting commands do not already wait for:

```bash
./scripts/browser-tools wait "#results"            # visible (default)
./scripts/browser-tools wait "#spinner" --absent   # also --hidden, --present
./scripts/browser-tools wait                       # until the page stops loading
```

## Capture

```bash
./scripts/browser-tools screenshot --output /tmp/shot.png   # --full-page or --selector <sel>
./scripts/browser-tools pdf --format letter --landscape --output ~/page.pdf
./scripts/browser-tools pick-element "Click the submit button"
```

`pick-element` asks the user to click elements in the page and returns them as JSON.

## Network

Both stream until Ctrl+C. Run them in a tmux pane or a background process.

```bash
./scripts/browser-tools network --type fetch --show-body --filter "api\.example\.com"
./scripts/browser-tools intercept block --url "*doubleclick*"
./scripts/browser-tools intercept redirect https://example.com/mock --url "*/api/*"
./scripts/browser-tools intercept mock --url "*/api/data" --status 200 --body '{"ok":true}'
./scripts/browser-tools intercept modify --set-header "Authorization: Bearer x"
```

## Browser data

```bash
./scripts/browser-tools clear --all                 # current origin
./scripts/browser-tools clear --cookies --cache --all-origins
./scripts/browser-tools clear --local-storage --session-storage --indexeddb
```

## Extensions

Needs a Chrome for Testing build (`--browser cft-stable`, installed via `update-cft`).

```bash
./scripts/browser-tools extension load ./my-extension    # prints the ID
./scripts/browser-tools extension reload <id>            # pick up code changes
./scripts/browser-tools extension list
./scripts/browser-tools extension action <id>            # click the toolbar action
./scripts/browser-tools extension uninstall <id>
```

To work on a popup or options page, open it as a normal tab with `navigate chrome-extension://<id>/popup.html`. Every command then applies to it.

## JavaScript

```bash
./scripts/browser-tools evaluate-js "document.querySelectorAll('a').length"
./scripts/browser-tools evaluate-js path/to/script.js
./scripts/browser-tools evaluate-js - <<'EOF'
(function() { return [...document.querySelectorAll('.item')].map(e => e.dataset.id); })()
EOF
```

A top-level `return` is a syntax error, so wrap statements in `(function(){ … })()`.

## Launch flags

Commands start the browser on their own. `start` does it explicitly and can pass Chrome flags, which only apply to a fresh launch:

```bash
./scripts/browser-tools start --fake-media            # fake camera + mic, permission granted without prompt
./scripts/browser-tools start --arg=--disable-backgrounding-occluded-windows # any Chrome flag, repeatable
./scripts/browser-tools start --arg=-AppleLanguages --arg='(de)'   # UI language on macOS
```

With a browser already on the port, `start` with flags fails; run `./scripts/browser-tools stop` first. `getUserMedia` still needs HTTPS or `http://localhost`.

## Global options

Accepted before or after the command:

```bash
--browser <variant>   # chrome-stable|beta|dev|canary, cft-stable|beta|dev|canary
--port <n>            # remote debugging port (default 9222)
--timeout <duration>  # how long to wait for elements and loads (default 10s)
--dialog <answer>     # alert/confirm/prompt: accept (default), dismiss, or the text for a prompt
```

Every answered dialog is reported as a `dialog:` line.

## More

Full API details: [REFERENCE.md](REFERENCE.md).
