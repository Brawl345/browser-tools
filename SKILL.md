---
name: browser-tools
description: Interact with a web browser. Can start a browser, connect to it, evaluate JavaScript, make screenshots, read console logs and let the user select DOM elements. Use when interacting with unknown websites (e.g. scraping or Userscripts) or debugging browser-stuff. Requires uv.
---

# Browser Tools

This skill provides various scripts to interact with a web browser. These scripts can be run from anywhere, you only need to use the full path to this file's directory, but NEVER change the working directory. There is also no need to use "sleep" since all scripts will wait automatically.

## Start

Always start Chrome with remote debugging first:

```bash
uv run scripts/start.py
```

## Navigate to a web page

```bash
uv run scripts/navigate.py https://example.com
# or open a new tab
uv run scripts/navigate.py https://example.com --new
```

## Execute JavaScript

```bash
uv run scripts/evaluate.py "document.querySelectorAll('a').length"
# For complex and longer scripts, always use a file first
uv run scripts/evaluate.py path/to/script.js
```

## Pick DOM elements

Use an interactive element picker to instruct the user to pick DOM elements that should be debugged or shown:

```bash
uv run scripts/pick.py "Click the submit button"
uv run scripts/pick.py "Select all product cards"
```

Returns element information including tag, id, class, text content, HTML, and parent hierarchy.

## Mouse actions

```bash
uv run scripts/mouse.py click "button#submit"
uv run scripts/mouse.py dblclick ".item"
uv run scripts/mouse.py hover "nav .menu-item"
uv run scripts/mouse.py right-click ".context-menu-trigger"
uv run scripts/mouse.py drag ".draggable" --to ".drop-zone"
```

## Fill text fields

```bash
uv run scripts/fill.py "input#username" "john_doe"
uv run scripts/fill.py "textarea#comment" "Hello, world!" --clear
uv run scripts/fill.py "input[name='email']" "user@example.com"
```

## Check/uncheck checkboxes

```bash
uv run scripts/check.py "input#accept-terms"
uv run scripts/check.py "input[name='newsletter']" --uncheck
uv run scripts/check.py "input[type='radio'][value='option1']"
```

## Press keyboard keys

```bash
uv run scripts/press-key.py "Enter"
uv run scripts/press-key.py "Escape"
uv run scripts/press-key.py "a" --selector "input#search"
```

## Upload files

```bash
uv run scripts/upload.py "input[type='file']" /path/to/file.pdf
uv run scripts/upload.py "#file-upload" /path/to/image1.jpg /path/to/image2.png
```

## Download files

```bash
uv run scripts/download.py "a[href='/report.pdf']"
uv run scripts/download.py "button#download" --output ~/Documents/report.pdf
```

## Select dropdown options

```bash
uv run scripts/select-dropdown.py "select#country" "US"
uv run scripts/select-dropdown.py "select[name='color']" "Red" --by-label
uv run scripts/select-dropdown.py "#quantity" "2" --by-index
```

## Cookies

### List cookies

```bash
uv run scripts/cookies.py
```

### Clear cookies

```bash
uv run scripts/clear-cookies.py
```

## Local/Session storage

### List storage items

```bash
uv run scripts/storage.py
```

### Clear local/session storage

```bash
uv run scripts/clear-storage.py
```

## Get console messages

```bash
uv run scripts/console.py
uv run scripts/console.py --errors-only
```

## Capture network requests

ALWAYS start this script in a background agent. During this time, you can manually interact with the page to trigger network requests. The script then logs all requests made.

```bash
uv run scripts/network.py
uv run scripts/network.py --type fetch --show-body
uv run scripts/network.py --filter "api\\.example\\.com" --show-headers
```

## Get HTML content

Outputs HTML with optional filtering and additional context.

```bash
uv run scripts/get-html.py
uv run scripts/get-html.py --filter "<button.*submit.*>"
uv run scripts/get-html.py --filter "data-id=\"\d+\"" --lines 10
```

## Manage tabs

List all open tabs, switch to a specific tab, or close a tab:

```bash
uv run scripts/tabs.py
uv run scripts/tabs.py --switch 0
uv run scripts/tabs.py --close 1
```

## More

For detailed API reference, see [REFERENCE.md](REFERENCE.md).
