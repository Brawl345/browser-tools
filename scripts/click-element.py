#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright",
# ]
# ///

import asyncio
import click as click_lib
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click_lib.command()
@click_lib.argument("selector")
@click_lib.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click_lib.option(
    "--timeout",
    type=int,
    default=10000,
    help="Timeout in milliseconds (default: 10000)"
)
@click_lib.option(
    "--force",
    is_flag=True,
    help="Force click even if element is not visible or enabled"
)
def main(selector, port, timeout, force):
    """Click on an element using a CSS selector.

    Example:
      click-element.py "button#submit"
      click-element.py ".product-card:first-child"
      click-element.py "a[href='/login']" --timeout 5000
      click-element.py "#hidden-button" --force
    """
    asyncio.run(click_element(selector, port, timeout, force))

async def click_element(selector, port, timeout, force):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Connected to page: {page.url}")
            click_lib.echo(f"Looking for element: {selector}")

            if force:
                await page.locator(selector).click(force=True, timeout=timeout)
            else:
                await page.locator(selector).click(timeout=timeout)

            click_lib.echo(f"Successfully clicked element: {selector}")

        except Exception as e:
            click_lib.echo(f"Failed to click element: {e}", err=True)

if __name__ == "__main__":
    main()
