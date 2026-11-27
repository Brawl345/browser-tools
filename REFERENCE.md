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

Every command starts the browser automatically if it isn't running yet.

```bash
# Use a specific browser variant or port
./scripts/browser-tools --browser chrome-canary --port 9222 start
# Or via environment variable
BROWSER_TOOLS_BROWSER=chrome-canary ./scripts/browser-tools start
```

`start` passes extra flags to Chrome. They only take effect when `start` launches the browser; if one is already listening on the port, `start` with flags exits with an error. Quit that browser first. Another `--port` does not help: every variant keeps one profile directory, and a second launch on the same profile hands over to the running instance.

| Flag | Description |
|---|---|
| `--fake-media` | Adds `--use-fake-device-for-media-stream` and `--use-fake-ui-for-media-stream`: `getUserMedia` gets a synthetic camera (green test pattern with a running clock) and microphone, and the permission is granted without a prompt |
| `--arg <flag>` | Appends any Chrome command-line flag; repeatable. Write it as `--arg=--flag=value` |

```bash
./scripts/browser-tools start --fake-media
./scripts/browser-tools start --arg=--force-dark-mode
```

For changing the UI language use `--arg=--lang=de` on Linux and Windows. On macOS, use `AppleLanguages` passed as two arguments: `--arg=-AppleLanguages --arg='(de)'`.

## Global Options

These flags are accepted before or after the command name, so `wait #x --timeout 30s` and `--timeout 30s wait #x` mean the same thing:

| Flag | Default | Description |
|---|---|---|
| `--browser` | `chrome-stable` | Browser variant: `chrome-stable`, `chrome-beta`, `chrome-dev`, `chrome-canary`, `cft-stable`, `cft-beta`, `cft-dev`, `cft-canary` |
| `--port` | `9222` | Remote debugging port |
| `--timeout` | `10s` | How long to wait for elements and page loads (e.g. `30s`, `500ms`, `2m`) |

---

## Selectors

Wherever a command takes a selector, as an argument or via `--selector`/`--to`, it accepts three kinds, told apart by their first characters:

| Form | Example | Matched against |
|---|---|---|
| CSS | `button#submit` | the document and every open shadow root |
| XPath (`/`, `./`, `(/`, `(./`) | `//tr[3]//a` | the main document |
| Text (`text=`) | `text=Sign in` | visible text, `value`, `aria-label`, `title`, `placeholder`, `alt` |

```bash
./scripts/browser-tools mouse click "text=Sign in"
./scripts/browser-tools element "text=Imprint" --attr href
./scripts/browser-tools fill "text=E-Mail" "me@example.com"
./scripts/browser-tools wait "//div[@role='dialog']" --absent
```

`text=` compares case-insensitively after collapsing whitespace. An exact label beats a partial one, the innermost element carrying the text wins over its wrappers, and a visible match wins over a hidden one. It also reads `value` and `placeholder`, so it addresses `<input type="button" value="Install">`, which XPath's `normalize-space()` never matches. On a page that ships in several languages the label changes with the locale; a CSS selector holds up better there.

A CSS selector is tried on the document first and only falls back to shadow roots when nothing matches there, so pages without web components pay nothing for it.

An XPath must select element nodes. `//h1/text()` and `//a/@href` end in an error that names the cause; read attributes with `element --attr` instead.

## Waiting and failures

Every selector is polled until it reaches the wanted state or `--timeout` runs out. While it polls, it also re-checks which tab is in front, so an element that appears in a tab opened during the wait is still found. That is what makes extension and OAuth dialogs work without a detour through `tab`.

Once the element exists it is scrolled into view, which lets players and lazily rendered sections finish building. If nothing matches at all, the page is walked from top to bottom to wake up content that only renders near the viewport, and scrolled back when the search fails anyway.

A failed selector reports what was wrong instead of a bare timeout:

```
error: nothing matches text=Instal Now
  tab: Script Installation | Tampermonkey — https://www.tampermonkey.net/…
  texts on the page: "Neu installieren", "Abbrechen", "Tampermonkey®"
  2 tabs are open — check `browser-tools tab` if this is the wrong one
```

```
error: #overlay-button matches 1 element(s) but none is visible
  tab: Example — https://example.com/
  the <div> is there but display:none, box is 0x0
  covered by div.modal-backdrop — click that instead, or pass --force
```

For a CSS selector that matches nothing, the report names the longest prefix that still matches something, so it is clear which part of the chain is wrong.

## Settling

`navigate` and `reload` return once the page has fired its load event and traffic has died down; pages that poll forever are not waited on beyond a short cap. The acting commands (`mouse`, `fill`, `check`, `select`, `key`, `scroll`) wait for the page to go quiet after they act. Scripting `sleep` around these commands is never needed.

## Commands

### navigate

Navigate to a URL in the active tab or open in a new tab. A URL without a scheme gets `https://`, or `http://` for `localhost` and `127.0.0.1`. `about:`, `data:` and `file://` URLs work as they are:

```bash
./scripts/browser-tools navigate https://example.com
./scripts/browser-tools navigate example.com --new-tab
```

The command returns once the page has loaded. When the load event has not arrived by `--timeout` it still succeeds and says `(still loading)`, so a slow page never costs the whole command.

Options:
- `--new-tab`: Open the URL in a new tab instead of reusing the current one

### reload

Reload the active tab and wait for it to load, the same way `navigate` does:

```bash
./scripts/browser-tools reload
./scripts/browser-tools reload --hard
```

Options:
- `--hard`: Bypass the HTTP cache

### evaluate-js

NOTE: Always prefer the specialised commands. When a script does something a command covers, the run prints a `hint:` line naming that command.

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
./scripts/browser-tools screenshot --selector "text=Total price"
./scripts/browser-tools screenshot --output /tmp/shot.png
```

An element capture clips to the element's box in page coordinates, so it also works for elements taller than the viewport and for elements inside shadow roots.

Options:
- `--full-page`: Capture the entire page, not just the viewport
- `--selector <sel>`: Capture only the element matching the selector (mutually exclusive with `--full-page`)
- `--output <path>`: Custom output path (parent directories are created if needed)

### pdf

Save the current page as a PDF via `Page.printToPDF`, written to `/tmp/page-YYYYMMDD-HHMMSS.pdf` by default. Prints the path to stdout.

```bash
./scripts/browser-tools pdf
./scripts/browser-tools pdf --format letter --landscape
./scripts/browser-tools pdf --output ~/Documents/report.pdf
./scripts/browser-tools pdf --margin 0.2 --header-footer
./scripts/browser-tools pdf --page-ranges "1-3, 5"
```

Options:
- `--output <path>`: Custom output path (parent directories are created if needed)
- `--format <name>`: Paper format: `a3`, `a4` (default), `a5`, `letter`, `legal`, `tabloid`
- `--landscape`: Use landscape orientation
- `--margin <inches>`: Page margin applied to all sides (default `0.4`)
- `--scale <float>`: Scale of the page rendering (default `1.0`)
- `--no-background`: Do not print background graphics (printed by default)
- `--header-footer`: Show a header (page title) and footer (page number) on each page
- `--header-template <html>`: Custom HTML header template (implies `--header-footer`)
- `--footer-template <html>`: Custom HTML footer template (implies `--header-footer`)
- `--page-ranges <ranges>`: Pages to print, one-based, e.g. `"1-5, 8, 11-13"`

Header and footer templates may use `<span>` elements with the classes `date`, `title`, `url`, `pageNumber` and `totalPages` to inject printing values. Headers and footers are only visible when the corresponding margin leaves room for them.

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

Emulate a viewport size, or a phone, for the current tab:

```bash
./scripts/browser-tools resize 1280 720
# Phone: mobile viewport, touch input, 3x pixel ratio
./scripts/browser-tools resize --mobile --dpr 3 393 852
./scripts/browser-tools resize --mobile --dpr 3 --user-agent "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1" 393 852
# Clear the emulation and restore the window's own size
./scripts/browser-tools resize --reset
```

The emulation stays on the tab across commands, reloads and navigations until `resize --reset`, another `resize`, or the tab closes. A background `browser-tools emulation-holder` process keeps it alive.

Width and height are in CSS pixels and must be positive integers. With `--mobile`, a page without `<meta name="viewport">` lays out at 980px wide like on a real phone, and `mouse click`/`dblclick` send touch taps.

Options:
- `--mobile`: Mobile viewport and touch input (`navigator.maxTouchPoints`, `(pointer: coarse)`, taps)
- `--dpr <n>`: Device pixel ratio (default: the screen's own)
- `--user-agent <ua>`: User agent string; requests use it after the next reload
- `--reset`: Clear the emulation and restore the default size

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

Read everything about an element as JSON, without writing JavaScript. This is the command that replaces `getBoundingClientRect`, `getComputedStyle`, `textContent` and `querySelectorAll(...).length` scripts:

```bash
./scripts/browser-tools element "button#submit"
./scripts/browser-tools element "a.download" --attr href
./scripts/browser-tools element ".product .title" --attr text
```

The output always has the shape `{count, elements}`. `count` is the total number of matches.

On a single match the entry carries `tag`, `id`, `class`, `text` (max 200 chars), `value` (for form controls, read off the live property), `checked`, `disabled`, `attributes`, `box`, `visible`, `style`, `html` (outerHTML, max 500 chars) and `parents` (ancestor hierarchy). Empty fields are left out.

On several matches it prints up to 10 short entries with `tag`, `id`, `class`, `text`, `box` and `visible`, plus a `note` when more matched than were printed.

`style` holds `display`, `cursor`, `color`, `backgroundColor` and `fontSize`, plus `position`, `zIndex`, `opacity`, `visibility`, `pointerEvents` and `overflow` whenever those differ from their initial value.

Options:
- `--attr <name>`: Print one value per line as plain text, for up to 100 matches. Besides attribute names it understands `text`, `html` and `value`; for anything else the DOM property is used when the attribute is absent, so `--attr checked` and `--attr disabled` work too.

The command waits for the element to appear (up to `--timeout`) and reports what is on the page if it never does.

### wait

Block until an element reaches a state, then exit. The acting commands already wait by themselves, so this covers the states they do not: something disappearing, or the page simply going quiet. Without a selector it waits for the load event and for network traffic to stop:

```bash
./scripts/browser-tools wait "#results"
./scripts/browser-tools wait "#spinner" --hidden
./scripts/browser-tools wait "#late-content" --present --timeout 30s
./scripts/browser-tools wait "#spinner" --absent
./scripts/browser-tools wait
```

States (mutually exclusive):
- `--visible` (default): element exists and is visible
- `--hidden`: element exists in the DOM but is not visible
- `--present`: element exists in the DOM (visible or not)
- `--absent`: element does not exist in the DOM

### mouse

Simulate mouse actions on elements:

```bash
./scripts/browser-tools mouse click "text=Submit"
./scripts/browser-tools mouse dblclick ".item"
./scripts/browser-tools mouse hover "nav .menu-item"
./scripts/browser-tools mouse right-click ".context-menu-trigger"
./scripts/browser-tools mouse drag ".draggable" --to ".drop-zone"
# Force JS event dispatch for hidden/non-visible elements
./scripts/browser-tools mouse click "#hidden-button" --force
```

Without `--force` the events are real pointer events at the element's centre, which is what a page's own handlers see. When something else is drawn over that point, the confirmation says so, because that element is the one that got the click:

```
✓ click: .poster (div.overlay lies on top and takes the event)
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
- `--uncheck`: Uncheck the element (checkboxes only; a radio button cannot be unchecked)
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

Get browser console messages and uncaught exceptions from the current tab, including those logged before the command ran (Chrome keeps the last 1000 since the page loaded):

```bash
./scripts/browser-tools console
./scripts/browser-tools console --errors-only
```

Options:
- `--errors-only`: Only show errors and exceptions

### network

Capture network requests. Runs until Ctrl+C, so always start it in a tmux pane or a background process.

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

Block, redirect, or modify in-flight network requests. Runs until Ctrl+C, so always start it in a tmux pane or a background process.

The action is chosen by subcommand (`block`, `redirect`, `modify` or `mock`), so only the relevant options apply per run. Every action selects which requests to intercept with `--url` (wildcards `*` and `?`, default `*`) and optionally `--type`; matching requests are paused and the action is applied, while all other traffic passes through.

```bash
./scripts/browser-tools intercept block --url "*doubleclick*"
./scripts/browser-tools intercept redirect https://example.com/mock --url "*/api/*"
./scripts/browser-tools intercept modify --set-header "X-Test: 1" --remove-header "Cookie"
./scripts/browser-tools intercept mock --url "*/api/data" --status 200 --body '{"ok":true}' --content-type application/json
```

Shared selection options (all actions):
- `--url <pattern>`: Wildcard URL pattern to intercept (`*` = zero or more, `?` = exactly one; default `*`)
- `--type <t>`: Filter by resource type: `all`, `xhr`, `fetch`, `document`, `script`, `stylesheet`, `image`, `font`, `media`, `websocket`, `other` (default `all`)

`intercept block` fails matching requests with `ERR_BLOCKED_BY_CLIENT`. No extra options.

`intercept redirect <url>` reroutes matching requests to `<url>`. The page does not notice and the address bar is unchanged.

`intercept modify` adds, overrides or removes request headers, then continues:
- `--set-header "Name: Value"`: Add/override a request header (repeatable)
- `--remove-header <name>`: Remove a request header by name (repeatable)

`intercept mock` answers matching requests with a custom response without hitting the server:
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

---

### update-cft

Download or update a Chrome for Testing build into the local cache. Builds are
stored at `~/.cache/claude-browser-tools/chrome-for-testing-binaries/<channel>/`
and an update replaces the existing build.

```bash
./scripts/browser-tools update-cft                    # stable
./scripts/browser-tools update-cft --channel beta
./scripts/browser-tools update-cft --channel stable --check
```

Options:
- `--channel <c>`: `stable` (default), `beta`, `dev`, or `canary`
- `--check`: Only report whether an update is available
- `--force`: Reinstall even if already up to date

---

### extension

Manage unpacked extensions. Requires a Chrome for Testing variant; other variants report an error.

```bash
./scripts/browser-tools extension load ./my-extension
./scripts/browser-tools extension list
./scripts/browser-tools extension reload <id>
./scripts/browser-tools extension action <id>
./scripts/browser-tools extension uninstall <id>
```

Commands:
- `load <dir>`: Load an unpacked extension from a directory, printing its ID
- `list`: List loaded unpacked extensions with ID, name, version, and status
- `reload <id>`: Reload an extension from its source directory, picking up code changes
- `action <id>`: Trigger the extension's toolbar action on the active tab
- `uninstall <id>`: Remove a loaded extension

`action` presses the toolbar button, the same way a user opens the popup. Chrome closes an extension popup as soon as it loses focus, so drive its contents in a normal tab instead, where every command works on it as on any page:

```bash
./scripts/browser-tools navigate "chrome-extension://<id>/popup.html"
./scripts/browser-tools mouse click "#my-button"
```
