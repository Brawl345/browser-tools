# Playwright Tools

Python scripts for controlling Chrome via Playwright's CDP protocol.

## Prerequisites

Start Chrome with remote debugging:

```bash
uv run scripts/start.py
```

By default, this launches Chrome Canary. Use `--browser` to specify a different variant (chrome-stable, chrome-beta, chrome-dev, chrome-canary). There is no need to use "sleep" or equivalents since the scripts already try connecting multiple times with backoff.

To use a custom browser executable:

```bash
uv run scripts/start.py --path /path/to/chromium
```

**Platform Support:**
- **macOS**: Chrome variants installed in `/Applications/`
- **Windows**: Chrome variants in `%PROGRAMFILES%` or `%LOCALAPPDATA%`
- **Linux**: Chrome variants detected via `which` or in `/usr/bin/`

## Scripts

### Navigate

Navigate to a URL in the active tab or open in a new tab:

```bash
uv run scripts/navigate.py https://example.com
uv run scripts/navigate.py https://example.com --new
```

### Evaluate

Execute JavaScript on the current page. Accepts either inline code or a path to a .js file:

```bash
uv run scripts/evaluate.py "document.title"
uv run scripts/evaluate.py "document.querySelectorAll('a').length"
uv run scripts/evaluate.py "window.location.href"
uv run scripts/evaluate.py path/to/script.js
```

### Get HTML

Get the full HTML content of the current page, or filter with regex:

```bash
uv run scripts/get-html.py
uv run scripts/get-html.py --filter "search-string"
uv run scripts/get-html.py --filter "<button.*submit.*>"
uv run scripts/get-html.py --filter "data-id=\"\d+\"" --lines 10
```

With `--filter`, the script searches for the regex pattern and outputs matching lines with surrounding context (default: 5 lines before and after).

### Screenshot

Take a screenshot with timestamp:

```bash
uv run scripts/screenshot.py
```

Screenshots are saved to the system temp directory as `screenshot_YYYYMMDD_HHMMSS.png`.

### Cookies

List all cookies for the current site:

```bash
uv run scripts/cookies.py
```

### Pick Elements

Interactive element picker for scraping. Click to select, Cmd/Ctrl+Click for multi-select, Enter to finish:

```bash
uv run scripts/pick.py "Click the submit button"
uv run scripts/pick.py "Select all product cards"
```

Returns element information including tag, id, class, text content, HTML, and parent hierarchy.

### Click Element

Click on an element using a CSS selector:

```bash
uv run scripts/click-element.py "button#submit"
uv run scripts/click-element.py ".product-card:first-child"
uv run scripts/click-element.py "a[href='/login']"
uv run scripts/click-element.py "#hidden-button" --force
uv run scripts/click-element.py "button.load-more" --timeout 5000
```

Options:
- `--force`: Force click even if element is not visible or enabled
- `--timeout`: Timeout in milliseconds (default: 10000)

### Fill Text Fields

Fill input or textarea elements with text using a CSS selector:

```bash
uv run scripts/fill.py "input#username" "john_doe"
uv run scripts/fill.py "textarea#comment" "Hello, world!"
uv run scripts/fill.py "input[name='email']" "user@example.com" --clear
uv run scripts/fill.py "input.search" "search query" --timeout 5000
```

Options:
- `--clear`: Clear the field before filling
- `--timeout`: Timeout in milliseconds (default: 10000)

### Press Key

Press keyboard keys (Enter, Escape, Tab, etc.):

```bash
uv run scripts/press-key.py "Enter"
uv run scripts/press-key.py "Escape"
uv run scripts/press-key.py "Tab"
uv run scripts/press-key.py "a"
uv run scripts/press-key.py "a" --selector "input#search"
uv run scripts/press-key.py "Enter" --timeout 5000
```

Common keys: Enter, Escape, Tab, Backspace, Delete, ArrowLeft, ArrowRight, ArrowUp, ArrowDown, or any single character.

Options:
- `--selector`: Optional CSS selector to focus before pressing key
- `--timeout`: Timeout in milliseconds (default: 10000)

### Console

Get browser console messages and page errors (up to 200 most recent messages):

```bash
uv run scripts/console.py
uv run scripts/console.py --errors-only
```

The script displays:
- Console messages (log, info, warning, error, debug) with color coding
- Source location (file, line, column) for each message

Options:
- `--errors-only`: Only show errors and warnings

### Network

Capture network requests (XHR, Fetch, etc.):

```bash
uv run scripts/network.py
uv run scripts/network.py --type fetch
uv run scripts/network.py --type xhr --show-body
uv run scripts/network.py --filter "api\\.example\\.com"
uv run scripts/network.py --no-reload --duration 10
```

The script displays:
- HTTP method, status code, resource type, and URL for each request
- Request and response headers with `--show-headers`
- Request and response bodies (for fetch/xhr only) with `--show-body`

Options:
- `--type`: Filter by resource type (all, xhr, fetch, document, script, stylesheet, image, font, media)
- `--filter`: Filter URLs by regex pattern
- `--show-headers`: Show request and response headers
- `--show-body`: Show request and response bodies (only for fetch/xhr)
- `--no-reload`: Don't reload the page, just listen for new requests (default: false)
- `--duration`: Duration in seconds to listen when using `--no-reload` (default: 10)

## Options

All scripts support `--port` to specify a custom debugging port (default: 9222):

```bash
uv run scripts/navigate.py https://example.com --port 9223
```
