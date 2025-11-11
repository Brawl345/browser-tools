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
@click_lib.argument("text")
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
    "--clear",
    is_flag=True,
    help="Clear the field before filling"
)
def main(selector, text, port, timeout, clear):
    """Fill a text field using a CSS selector.

    Example:
      fill.py "input#username" "john_doe"
      fill.py "textarea#comment" "Hello, world!" --clear
      fill.py "input[name='email']" "user@example.com" --timeout 5000
    """
    asyncio.run(fill_field(selector, text, port, timeout, clear))

async def fill_field(selector, text, port, timeout, clear):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for field: {selector}")

            locator = page.locator(selector)

            if clear:
                await locator.clear(timeout=timeout)

            await locator.fill(text, timeout=timeout)

            click_lib.echo(f"Successfully filled field: {selector}")

        except Exception as e:
            click_lib.echo(f"Failed to fill field: {e}", err=True)

if __name__ == "__main__":
    main()
