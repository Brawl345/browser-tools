# Browser Tools Reference

Go binary for controlling Chrome via the Chrome DevTools Protocol.

## Prerequisites

**IMPORTANT:** Always prefix the binary path when running commands:

```bash
{basedir}/scripts/browser-tools <command> [options]
```

Start Chrome with remote debugging:

```bash
./scripts/browser-tools start
```

Every command starts the browser automatically if it isn't running yet. No need for explicit sleeps - commands wait for elements automatically.

```bash
# Use a specific browser variant or port
./scripts/browser-tools --browser chrome-canary --port 9222 start
# Or via environment variable
BROWSER_TOOLS_BROWSER=chrome-canary ./scripts/browser-tools start
```

## Global Options

All commands accept these flags **before** the command name:

| Flag | Default | Description |
|---|---|---|
| `--browser` | `chrome-canary` | Browser variant: `chrome-stable`, `chrome-beta`, `chrome-dev`, `chrome-canary` |
| `--port` | `9222` | Remote debugging port |
| `--timeout` | `10s` | Timeout for commands that wait on elements (e.g. `30s`, `500ms`, `2m`) |

---

## Commands

### navigate

Navigate to a URL in the active tab or open in a new tab:

```bash
./scripts/browser-tools navigate https://example.com
./scripts/browser-tools navigate https://example.com --new-tab
```

### evaluate-js

NOTE: Always prefer specialized commands like `html`, `mouse`, `network` or `element`/`pick-element` whenever possible.

Execute JavaScript on the current page. Accepts inline code, a path to a `.js` file, or `-` to read from STDIN.

**IMPORTANT:** Top-level `return` statements cause a `SyntaxError`. Always wrap multi-statement scripts in an IIFE: `(function() { ...; return result; })()`

```bash
./scripts/browser-tools evaluate-js "document.title"
./scripts/browser-tools evaluate-js "document.querySelectorAll('a').length"
# Multi-line via heredoc
./scripts/browser-tools evaluate-js - <<'EOF'
(function() {
  const title = document.querySelector('h1').textContent;
  const links = Array.from(document.querySelectorAll('a')).map(a => a.href);
  return {title, linkCount: links.length};
})()
EOF
# From a file
./scripts/browser-tools evaluate-js path/to/script.js
```

### html

Get the full HTML content of the current page, optionally filtered with a regex:

```bash
./scripts/browser-tools html
./scripts/browser-tools html --filter "<button.*submit.*>"
./scripts/browser-tools html --filter "data-id=\"\d+\"" --lines 10
```

With `--filter`, matching lines are shown with surrounding context (default: 5 lines before/after).

Options:
- `--filter <regex>`: Filter output by regex pattern
- `--lines <n>`: Number of context lines around each match (default: 5)

### screenshot

NOTE: Prefer `html` or `pick-element` over screenshots whenever possible to save on token usage.

Take a screenshot saved to `/tmp/screenshot-YYYYMMDD-HHMMSS.png`. Prints the path to stdout.

```bash
./scripts/browser-tools screenshot
./scripts/browser-tools screenshot --full-page
./scripts/browser-tools screenshot --selector ".chart"
```

Options:
- `--full-page`: Capture the entire page, not just the viewport
- `--selector <css>`: Capture only the element matching the CSS selector (mutually exclusive with `--full-page`)

### scroll

Scroll the page to an element or to an absolute position. Exactly one mode may be used at a time:

```bash
# Scroll an element into view (waits for it, up to --timeout)
./scripts/browser-tools scroll "#section"
# Scroll to an absolute X/Y position in pixels
./scripts/browser-tools scroll --x 0 --y 800
# Scroll to the very top or bottom of the page
./scripts/browser-tools scroll --top
./scripts/browser-tools scroll --bottom
```

When only one of `--x`/`--y` is given, the other axis keeps its current scroll position.

Options:
- `--x <n>`: Absolute X position in pixels
- `--y <n>`: Absolute Y position in pixels
- `--top`: Scroll to the top of the page
- `--bottom`: Scroll to the bottom of the page

### resize

Set the viewport size via `Emulation.setDeviceMetricsOverride`:

```bash
./scripts/browser-tools resize 1280 720
./scripts/browser-tools resize 375 812
# Clear the override and restore the default size
./scripts/browser-tools resize --reset
```

Width and height are in CSS pixels and must be positive integers.

Options:
- `--reset`: Clear the viewport override and restore the default size

### cookie

List cookies for the current tab:

```bash
./scripts/browser-tools cookie
./scripts/browser-tools cookie --all
```

Options:
- `--all`: List cookies from all origins (not just the current tab's URL)

### clear

Clear various types of browser data.

```bash
# Clear everything for the current origin
./scripts/browser-tools clear --all
# Individual types
./scripts/browser-tools clear --cookies
./scripts/browser-tools clear --cache
./scripts/browser-tools clear --local-storage
./scripts/browser-tools clear --session-storage
./scripts/browser-tools clear --indexeddb
./scripts/browser-tools clear --cache-storage
./scripts/browser-tools clear --service-workers
# All origins (applies to cookies; cache is always browser-wide; storage is always per-origin)
./scripts/browser-tools clear --all --all-origins
```

Options:
- `--cookies`: Clear cookies
- `--cache`: Clear HTTP cache (always browser-wide)
- `--local-storage`: Clear localStorage for the current origin
- `--session-storage`: Clear sessionStorage for the current origin
- `--indexeddb`: Clear IndexedDB for the current origin
- `--cache-storage`: Clear Cache API / service worker caches for the current origin
- `--service-workers`: Unregister service workers for the current origin
- `--all`: Shorthand for all of the above
- `--all-origins`: For cookies: clear from all origins; for cache: already global; storage is always per-origin

### dom-storage

Show localStorage and/or sessionStorage for the current tab:

```bash
./scripts/browser-tools dom-storage
./scripts/browser-tools dom-storage --local
./scripts/browser-tools dom-storage --session
```

Options:
- `--local`: Show localStorage only
- `--session`: Show sessionStorage only (mutually exclusive with `--local`)

### pick-element

Interactive element picker. Click to select, Cmd/Ctrl+Click for multi-select, Enter to confirm, Escape to cancel:

```bash
./scripts/browser-tools pick-element "Click the submit button"
./scripts/browser-tools pick-element "Select all product cards"
```

Returns a JSON array of the selected elements (always an array, even for a single selection), each with tag, id, class, text, attributes, bounding box, visibility, HTML, and parent hierarchy.

### element

Read the properties of a known CSS selector as JSON, without writing JavaScript. Useful for reading text/attributes and for getting an element's bounding box and visibility (e.g. before a `mouse` or `screenshot` action):

```bash
./scripts/browser-tools element "button#submit"
./scripts/browser-tools element "a.download" --attr href
```

The output always has the same shape, `{count, elements}`, regardless of how many elements match. `count` is the total number of matches and `elements` is an array holding a sample of the first few (max 3). When more than one element matches, a `note` field is added advising you to refine the selector to target a single element.

Each entry in `elements` has `tag`, `id`, `class`, `text` (trimmed, max 200 chars), `attributes` (all attributes as a map), `box` (`x`/`y`/`width`/`height`), `visible`, `html` (outerHTML, max 500 chars) and `parents` (ancestor hierarchy). Fields that are null/empty are omitted.

Options:
- `--attr <name>`: Print only that attribute's value as plain text (exit 1 if absent). With multiple matches it prints one value per line.

The command waits for the element to appear in the DOM (up to `--timeout`) and errors if it never does.

### wait

Block until an element matching a CSS selector reaches a state, then exit. Useful for synchronising on dynamic content before another action. The command waits up to `--timeout` and exits 1 if the state is never reached:

```bash
./scripts/browser-tools wait "#results"
./scripts/browser-tools wait "#spinner" --hidden
./scripts/browser-tools --timeout 30s wait "#late-content" --present
./scripts/browser-tools wait "#spinner" --absent
```

States (mutually exclusive):
- `--visible` (default): element exists and is visible
- `--hidden`: element exists in the DOM but is not visible
- `--present`: element exists in the DOM (visible or not)
- `--absent`: element does not exist in the DOM

### mouse

Simulate mouse actions on elements using CSS selectors:

```bash
./scripts/browser-tools mouse click "button#submit"
./scripts/browser-tools mouse dblclick ".item"
./scripts/browser-tools mouse hover "nav .menu-item"
./scripts/browser-tools mouse right-click ".context-menu-trigger"
./scripts/browser-tools mouse drag ".draggable" --to ".drop-zone"
# Force JS event dispatch for hidden/non-visible elements
./scripts/browser-tools mouse click "#hidden-button" --force
```

Actions:
- `click`: Left-click
- `dblclick`: Double-click
- `hover`: Move mouse over element
- `right-click`: Right-click (opens context menu)
- `drag`: Drag to another element (requires `--to`)

Options:
- `--to <selector>`: Target selector for drag (required for drag)
- `--force`: Dispatch JS mouse events even if element is not visible

### fill

Fill an input or textarea with text:

```bash
./scripts/browser-tools fill "input#username" "john_doe"
./scripts/browser-tools fill "textarea#comment" "Hello, world!"
./scripts/browser-tools fill "input[name='email']" "user@example.com" --clear
```

Options:
- `--clear`: Clear the field before typing

### check

Check or uncheck a checkbox, or select a radio button:

```bash
./scripts/browser-tools check "input#accept-terms"
./scripts/browser-tools check "input[name='newsletter']" --uncheck
./scripts/browser-tools check "input[type='radio'][value='option1']"
# Force for hidden elements (uses JS)
./scripts/browser-tools check "#hidden-checkbox" --force
```

Options:
- `--uncheck`: Uncheck the element (checkboxes only — cannot uncheck radio buttons)
- `--force`: Set value via JS even if element is not visible

### key

Simulate a key press, optionally on a focused element:

```bash
./scripts/browser-tools key "Enter"
./scripts/browser-tools key "Escape"
./scripts/browser-tools key "Tab"
./scripts/browser-tools key "a" --selector "input#search"
```

Common keys: `Enter`, `Escape`, `Tab`, `Backspace`, `Delete`, `ArrowLeft`, `ArrowRight`, `ArrowUp`, `ArrowDown`, or any single character.

Options:
- `--selector <sel>`: CSS selector to focus before pressing the key

### upload

Set files on a file input element. Works on hidden inputs too:

```bash
./scripts/browser-tools upload "input[type='file']" /path/to/file.pdf
./scripts/browser-tools upload "#file-upload" /path/to/image1.jpg /path/to/image2.png
```

All file paths are validated to exist before the upload is attempted. Relative paths are resolved to absolute automatically.

### download

Click a download link or button and save the file. Use `--timeout` for large files:

```bash
./scripts/browser-tools --timeout 60s download "a[href='/report.pdf']"
./scripts/browser-tools --timeout 60s download "button#download" --output ~/Documents/report.pdf
```

By default, files are saved to `~/Downloads/<suggested filename>`. Prints the final path to stdout.

Options:
- `--output <path>`: Custom output path (parent directories are created if needed)

### select

Select an option from a `<select>` dropdown:

```bash
./scripts/browser-tools select "select#country" "US"
./scripts/browser-tools select "select[name='color']" "Red" --by-label
./scripts/browser-tools select "#quantity" "2" --by-index
# Force for hidden elements
./scripts/browser-tools select "select.hidden" "opt1" --force
```

Options:
- `--by-label`: Select by visible label text instead of value attribute
- `--by-index`: Select by 0-based index
- `--force`: Apply even if element is not visible

### console

Get browser console messages from the current tab:

```bash
./scripts/browser-tools console
./scripts/browser-tools console --errors-only
```

Options:
- `--errors-only`: Only show errors and exceptions

### network

Capture network requests. **Blocking** — runs until Ctrl+C. Always run in a tmux pane or background agent/process - it blocks until CTRL+C.

```bash
./scripts/browser-tools network
./scripts/browser-tools network --type fetch
./scripts/browser-tools network --type xhr --show-body
./scripts/browser-tools network --filter "api\.example\.com" --show-headers
```

Options:
- `--type <t>`: Filter by resource type: `all`, `xhr`, `fetch`, `document`, `script`, `stylesheet`, `image`, `font`, `media`, `websocket`, `other` (default: `all`)
- `--filter <regex>`: Filter URLs by regex pattern
- `--show-headers`: Show request and response headers
- `--show-body`: Show response body (xhr/fetch only)

### intercept

Block, redirect, or modify in-flight network requests. **Blocking** — runs until Ctrl+C. Always run in a tmux pane or background agent/process.

The action is chosen by subcommand — `block`, `redirect`, `modify`, or `mock` — so only the relevant options apply per run. Every action selects which requests to intercept with `--url` (wildcards `*` and `?`, default `*`) and optionally `--type`; matching requests are paused and the action is applied, while all other traffic passes through.

```bash
./scripts/browser-tools intercept block --url "*doubleclick*"
./scripts/browser-tools intercept redirect https://example.com/mock --url "*/api/*"
./scripts/browser-tools intercept modify --set-header "X-Test: 1" --remove-header "Cookie"
./scripts/browser-tools intercept mock --url "*/api/data" --status 200 --body '{"ok":true}' --content-type application/json
```

Shared selection options (all actions):
- `--url <pattern>`: Wildcard URL pattern to intercept (`*` = zero or more, `?` = exactly one; default `*`)
- `--type <t>`: Filter by resource type: `all`, `xhr`, `fetch`, `document`, `script`, `stylesheet`, `image`, `font`, `media`, `websocket`, `other` (default `all`)

`intercept block` — fail matching requests (`ERR_BLOCKED_BY_CLIENT`). No extra options.

`intercept redirect <url>` — reroute matching requests to `<url>` (transparent to the page; the address bar is unchanged).

`intercept modify` — add/override or remove request headers, then continue:
- `--set-header "Name: Value"`: Add/override a request header (repeatable)
- `--remove-header <name>`: Remove a request header by name (repeatable)

`intercept mock` — answer matching requests with a custom response without hitting the server:
- `--status <code>`: Response status code (default `200`)
- `--body <string>`: Response body
- `--file <path>`: Response body read from a file
- `--content-type <mime>`: Content-type response header

### tab

List, activate, close, or refresh tabs. The active tab is marked with `▶`:

```bash
./scripts/browser-tools tab
./scripts/browser-tools tab --activate 2
./scripts/browser-tools tab --close 3
./scripts/browser-tools tab --refresh 1
```

Options:
- `--activate <n>`: Activate tab by 1-based index
- `--close <n>`: Close tab by 1-based index
- `--refresh <n>`: Refresh tab by 1-based index
