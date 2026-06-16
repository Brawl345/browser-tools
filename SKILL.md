---
name: browser-tools
description: Interact with a web browser. Can start a browser, connect to it, evaluate JavaScript, make screenshots, read console logs and let the user select DOM elements. Use when interacting with unknown websites (e.g. scraping or Userscripts) or debugging browser-stuff.
compatibility: Requires Chrome
---

# Browser Tools

This skill provides a compiled Go binary (`browser-tools`) to interact with a web browser via Chrome DevTools Protocol. Commands automatically wait for elements and start the browser if needed. The binary is located at the skill's base dir.

**IMPORTANT:** Always prefix the binary path when running commands:

```bash
{basedir}/scripts/browser-tools <command> [options]
```

## Start

Start Chrome with remote debugging (done automatically by every command, but can be done explicitly):

```bash
./scripts/browser-tools start
```

## Navigate to a web page

```bash
./scripts/browser-tools navigate https://example.com
# Open in a new tab
./scripts/browser-tools navigate https://example.com --new-tab
```

## Execute JavaScript

NOTE: Always prefer specialized commands like `html`, `mouse`, `network` or `element`/`pick-element` whenever possible.

**IMPORTANT:** Top-level `return` statements cause a `SyntaxError`. Always wrap multi-statement scripts in an IIFE: `(function() { ...; return result; })()`

```bash
./scripts/browser-tools evaluate-js "document.querySelectorAll('a').length"
# Multi-line via heredoc
./scripts/browser-tools evaluate-js - <<'EOF'
(function() {
  const elements = document.querySelectorAll('.item');
  return elements.length;
})()
EOF
# From a file
./scripts/browser-tools evaluate-js path/to/script.js
```

## Pick DOM elements

Instruct the user to interactively pick a DOM element:

```bash
./scripts/browser-tools pick-element "Click the submit button"
./scripts/browser-tools pick-element "Select all product cards"
```

Returns a JSON array of selected elements, each with tag, id, class, text, attributes, bounding box, visibility, HTML and parent hierarchy.

## Read element info

Read a known CSS selector's properties as JSON — without writing JS. Returns tag, id, class, text, attributes, bounding box and visibility:

```bash
./scripts/browser-tools element "button#submit"
# Print only one attribute's value as plain text
./scripts/browser-tools element "a.download" --attr href
```

Always prints `{count, elements}` (same shape regardless of how many match). `count` is the total number of matches; `elements` holds a sample of the first few. On multiple matches a `note` field advises refining the selector.

## Mouse actions

```bash
./scripts/browser-tools mouse click "button#submit"
./scripts/browser-tools mouse dblclick ".item"
./scripts/browser-tools mouse hover "nav .menu-item"
./scripts/browser-tools mouse right-click ".context-menu-trigger"
./scripts/browser-tools mouse drag ".draggable" --to ".drop-zone"
# Force JS dispatch for hidden elements
./scripts/browser-tools mouse click ".hidden-btn" --force
```

## Fill text fields

```bash
./scripts/browser-tools fill "input#username" "john_doe"
./scripts/browser-tools fill "textarea#comment" "Hello, world!" --clear
```

## Check/uncheck checkboxes and radio buttons

```bash
./scripts/browser-tools check "input#accept-terms"
./scripts/browser-tools check "input[name='newsletter']" --uncheck
./scripts/browser-tools check "input[type='radio'][value='option1']"
# Force for hidden elements
./scripts/browser-tools check "input#hidden" --force
```

## Press keyboard keys

Common keys: `Enter`, `Escape`, `Tab`, `Backspace`, `Delete`, `ArrowLeft`, `ArrowRight`, `ArrowUp`, `ArrowDown`

```bash
./scripts/browser-tools key "Enter"
./scripts/browser-tools key "Escape"
./scripts/browser-tools key "a" --selector "input#search"
```

## Upload files

```bash
./scripts/browser-tools upload "input[type='file']" /path/to/file.pdf
./scripts/browser-tools upload "#file-upload" /path/to/image1.jpg /path/to/image2.png
```

## Download files

```bash
./scripts/browser-tools --timeout 60s download "a[href='/report.pdf']"
./scripts/browser-tools --timeout 60s download "button#download" --output ~/Documents/report.pdf
```

## Select dropdown options

```bash
./scripts/browser-tools select "select#country" "US"
./scripts/browser-tools select "select[name='color']" "Red" --by-label
./scripts/browser-tools select "#quantity" "2" --by-index
```

## Cookies

### List cookies

```bash
./scripts/browser-tools cookie
# All cookies from all origins
./scripts/browser-tools cookie --all
```

### Clear cookies

```bash
./scripts/browser-tools clear --cookies
# All origins
./scripts/browser-tools clear --cookies --all-origins
```

## Local/Session storage

### Show storage

```bash
./scripts/browser-tools dom-storage
./scripts/browser-tools dom-storage --local
./scripts/browser-tools dom-storage --session
```

### Clear storage

```bash
./scripts/browser-tools clear --local-storage
./scripts/browser-tools clear --session-storage
```

## Clear browser data

```bash
# Clear everything for the current origin
./scripts/browser-tools clear --all
# Individual types
./scripts/browser-tools clear --cookies --cache
./scripts/browser-tools clear --local-storage --session-storage --indexeddb
./scripts/browser-tools clear --cache-storage --service-workers
# All origins (cookies and cache only — storage is always per-origin)
./scripts/browser-tools clear --all --all-origins
```

## Get console messages

```bash
./scripts/browser-tools console
./scripts/browser-tools console --errors-only
```

## Capture network requests

ALWAYS run this command in a tmux pane or background process — it blocks until Ctrl+C.

```bash
./scripts/browser-tools network
./scripts/browser-tools network --type fetch --show-body
./scripts/browser-tools network --filter "api\.example\.com" --show-headers
```

## Get HTML content

```bash
./scripts/browser-tools html
./scripts/browser-tools html --filter "<button.*submit.*>"
./scripts/browser-tools html --filter "data-id=\"\d+\"" --lines 10
```

## Screenshots

```bash
./scripts/browser-tools screenshot
./scripts/browser-tools screenshot --full-page
```

## Scroll

```bash
./scripts/browser-tools scroll "#section"      # scroll element into view
./scripts/browser-tools scroll --x 0 --y 800   # scroll to X/Y position
./scripts/browser-tools scroll --top
./scripts/browser-tools scroll --bottom
```

## Manage tabs

```bash
./scripts/browser-tools tab
./scripts/browser-tools tab --activate 2
./scripts/browser-tools tab --close 3
./scripts/browser-tools tab --refresh 1
```

## Global options

All commands support these flags (placed before the command):

```bash
--browser string     # chrome-stable, chrome-beta, chrome-dev, chrome-canary (default: chrome-canary)
--port int           # remote debugging port (default: 9222)
--timeout duration   # timeout for element-waiting commands (default: 10s)
                     # increase for slow pages or large downloads, e.g. --timeout 60s
```

## More

For detailed API reference, see [REFERENCE.md](REFERENCE.md).

