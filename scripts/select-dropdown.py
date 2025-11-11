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
@click_lib.argument("value")
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
    "--by-label",
    is_flag=True,
    help="Select by visible label instead of value"
)
@click_lib.option(
    "--by-index",
    is_flag=True,
    help="Select by index (0-based)"
)
def main(selector, value, port, timeout, by_label, by_index):
    """Select an option from a dropdown using a CSS selector.

    Example:
      select-dropdown.py "select#country" "US"
      select-dropdown.py "select[name='color']" "Red" --by-label
      select-dropdown.py "#quantity" "2" --by-index
    """
    asyncio.run(select_dropdown(selector, value, port, timeout, by_label, by_index))

async def select_dropdown(selector, value, port, timeout, by_label, by_index):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for dropdown: {selector}")

            locator = page.locator(selector)

            if by_index:
                await locator.select_option(index=int(value), timeout=timeout)
                click_lib.echo(f"Successfully selected index {value} in dropdown: {selector}")
            elif by_label:
                await locator.select_option(label=value, timeout=timeout)
                click_lib.echo(f"Successfully selected label '{value}' in dropdown: {selector}")
            else:
                await locator.select_option(value=value, timeout=timeout)
                click_lib.echo(f"Successfully selected value '{value}' in dropdown: {selector}")

        except Exception as e:
            click_lib.echo(f"Failed to select dropdown option: {e}", err=True)

if __name__ == "__main__":
    main()
