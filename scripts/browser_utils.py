from playwright.async_api import async_playwright, Browser, Page, BrowserContext
from typing import Optional
import click
import os


async def connect_to_browser(port: int = 9222) -> Optional[Browser]:
    os.environ["NODE_OPTIONS"] = os.environ.get("NODE_OPTIONS", "") + " --no-deprecation"
    p = await async_playwright().start()
    try:
        browser = await p.chromium.connect_over_cdp(f"http://localhost:{port}")
        return browser
    except Exception as e:
        click.echo(f"Failed to connect to browser: {e}", err=True)
        return None


async def get_context(browser: Browser) -> Optional[BrowserContext]:
    contexts = browser.contexts
    if not contexts:
        click.echo("No browser contexts found", err=True)
        return None
    return contexts[0]


async def get_active_page(context: BrowserContext) -> Optional[Page]:
    pages = context.pages
    if not pages:
        click.echo("No pages found", err=True)
        return None

    page = None
    for p in reversed(pages):
        if not p.url.startswith("chrome://"):
            page = p
            break

    if not page:
        page = await context.new_page()
        await page.goto("about:blank")

    return page


async def get_browser_and_page(port: int = 9222) -> tuple[Optional[Browser], Optional[Page]]:
    browser = await connect_to_browser(port)
    if not browser:
        return None, None

    context = await get_context(browser)
    if not context:
        return browser, None

    page = await get_active_page(context)

    if page:
        click.echo(f"Connected to page: {page.url}")

    return browser, page
