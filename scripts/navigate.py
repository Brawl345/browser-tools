#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright",
# ]
# ///

import asyncio
import click
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page, get_context

@click.command()
@click.argument("url")
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--new",
    is_flag=True,
    help="Open URL in a new tab"
)
def main(url, port, new):
    """Navigate to a specific URL in an existing Chrome instance."""
    asyncio.run(navigate_to_url(url, port, new))

async def navigate_to_url(url, port, new):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser:
                return

            if new:
                context = await get_context(browser)
                if not context:
                    return
                page = await context.new_page()
            elif not page:
                return

            await page.goto(url, wait_until="domcontentloaded")
            click.echo(f"Navigated to {url}")
        except Exception as e:
            click.echo(f"Failed to connect or navigate: {e}", err=True)

if __name__ == "__main__":
    main()