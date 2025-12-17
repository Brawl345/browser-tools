# Browser Tools

Python scripts for controlling Chrome via Playwright's CDP protocol.

Always change the working directory to the project root before running any script.

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

NOTE: Prefer `get-html` or `pick` whenever possible to save on latency and token usage.

Execute JavaScript on the current page. Accepts either inline code or a path to a .js file:

```bash
uv run scripts/evaluate.py "document.title"
uv run scripts/evaluate.py "document.querySelectorAll('a').length"
uv run scripts/evaluate.py "window.location.href"
# For multi-line scripts, it's preferable to use a temp file
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

NOTE: Prefer `get-html` or `pick` over analyzing screenshots whenever possible to save on token usage. 

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

#### Clear Cookies

Clear cookies from the browser:

```bash
uv run scripts/clear-cookies.py
uv run scripts/clear-cookies.py --all
```

By default, only cookies for the current page are cleared. Use `--all` to clear all cookies from all origins.

Options:
- `--all`: Clear all cookies from all origins instead of just the current page

### Storage

List localStorage and/or sessionStorage from the current page:

```bash
uv run scripts/storage.py
uv run scripts/storage.py --local
uv run scripts/storage.py --session
```

By default, shows both localStorage and sessionStorage.

Options:
- `--local`: Show localStorage only
- `--session`: Show sessionStorage only
- `--all`: Show both (default)

#### Clear Storage

Clear localStorage and/or sessionStorage from the current page:

```bash
uv run scripts/clear-storage.py
uv run scripts/clear-storage.py --local
uv run scripts/clear-storage.py --session
```

By default, both localStorage and sessionStorage are cleared.

Options:
- `--local`: Clear localStorage only
- `--session`: Clear sessionStorage only
- `--all`: Clear both (default)

### Pick Elements

Interactive element picker for scraping. Click to select, Cmd/Ctrl+Click for multi-select, Enter to finish:

```bash
uv run scripts/pick.py "Click the submit button"
uv run scripts/pick.py "Select all product cards"
```

Returns element information including tag, id, class, text content, HTML, and parent hierarchy.

### Mouse Actions

Perform various mouse actions on elements using CSS selectors:

```bash
uv run scripts/mouse.py click "button#submit"
uv run scripts/mouse.py dblclick ".item"
uv run scripts/mouse.py hover "nav .menu-item"
uv run scripts/mouse.py right-click ".context-menu-trigger"
uv run scripts/mouse.py drag ".draggable" --to ".drop-zone"
uv run scripts/mouse.py click "#hidden-button" --force
uv run scripts/mouse.py click "button.load-more" --timeout 5000
uv run scripts/mouse.py click "button" --delay 100
```

Actions:
- `click`: Left-click on an element
- `dblclick`: Double-click on an element
- `hover`: Hover over an element
- `right-click`: Right-click on an element (opens context menu)
- `drag`: Drag an element to another element (requires `--to` selector)

Options:
- `--to`: Target selector for drag action (required for drag)
- `--force`: Force action even if element is not visible or enabled
- `--timeout`: Timeout in milliseconds (default: 10000)
- `--delay`: Delay between mousedown and mouseup in milliseconds

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

### Check/Uncheck Elements

Check or uncheck checkboxes and select radio buttons using a CSS selector:

```bash
uv run scripts/check.py "input#accept-terms"
uv run scripts/check.py "input[name='newsletter']" --uncheck
uv run scripts/check.py "input[type='radio'][value='option1']"
uv run scripts/check.py "#hidden-checkbox" --force
uv run scripts/check.py "input.terms" --timeout 5000
```

Options:
- `--uncheck`: Uncheck the checkbox (only works for checkboxes, not radio buttons)
- `--force`: Force check even if element is not visible or enabled
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

### Upload Files

Upload single or multiple files to a file input using a CSS selector:

```bash
uv run scripts/upload.py "input[type='file']" /path/to/file.pdf
uv run scripts/upload.py "#file-upload" /path/to/image1.jpg /path/to/image2.png
uv run scripts/upload.py "input[name='document']" ~/Documents/report.pdf
uv run scripts/upload.py ".file-input" /path/to/file.txt --timeout 60000
```

The script automatically validates that all files exist before attempting to upload them. It supports both absolute and relative paths, as well as paths with tilde (~) for home directory.

Options:
- `--timeout`: Timeout in milliseconds (default: 30000)

### Download Files

Click a download link or button and save the downloaded file:

```bash
uv run scripts/download.py "a[href='/report.pdf']"
uv run scripts/download.py "button#download"
uv run scripts/download.py ".download-button" --output ~/Documents/report.pdf
uv run scripts/download.py "a.export" --timeout 60000
```

By default, files are saved to `~/Downloads/` with their suggested filename. Use `--output` to specify a custom path.

Options:
- `--output`: Custom output path for the downloaded file (creates parent directories if needed)
- `--timeout`: Timeout in milliseconds (default: 30000)

### Select Dropdown

Select an option from a dropdown menu using a CSS selector:

```bash
uv run scripts/select-dropdown.py "select#country" "US"
uv run scripts/select-dropdown.py "select[name='color']" "Red" --by-label
uv run scripts/select-dropdown.py "#quantity" "2" --by-index
uv run scripts/select-dropdown.py "select.product-options" "medium" --timeout 5000
```

Options:
- `--by-label`: Select by visible label text instead of value attribute
- `--by-index`: Select by zero-based index position
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

Capture network requests (XHR, Fetch, etc.). This script is **blocking** and MUST be started in a background agent.

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

### Tabs

List all open tabs and their URLs, switch to a specific tab, or close a tab by index:

```bash
uv run scripts/tabs.py
uv run scripts/tabs.py --switch 0
uv run scripts/tabs.py --close 1
```

The script displays:
- Tab index (0-based)
- Tab title
- Tab URL

When switching tabs, the specified tab is brought to the front, which makes `get_active_page()` return that tab for subsequent operations.

Options:
- `--switch`: Tab index to switch to (0-based)
- `--close`: Tab index to close (0-based)
- If neither option is provided, lists all tabs instead
- Cannot use `--switch` and `--close` together

## Global Options

All scripts support `--port` to specify a custom debugging port (default: 9222):

```bash
uv run scripts/navigate.py https://example.com --port 9223
```
