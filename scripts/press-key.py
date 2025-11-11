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
@click_lib.argument("key")
@click_lib.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click_lib.option(
    "--selector",
    help="Optional CSS selector to focus before pressing key"
)
@click_lib.option(
    "--timeout",
    type=int,
    default=10000,
    help="Timeout in milliseconds (default: 10000)"
)
def main(key, port, selector, timeout):
    """Press a keyboard key.

    Example:
      press-key.py "Enter"
      press-key.py "Escape"
      press-key.py "Tab"
      press-key.py "a" --selector "input#search"

    Common keys: Enter, Escape, Tab, Backspace, Delete, ArrowLeft, ArrowRight, ArrowUp, ArrowDown
    """
    asyncio.run(press_key(key, port, selector, timeout))

async def press_key(key, port, selector, timeout):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Connected to page: {page.url}")

            if selector:
                click_lib.echo(f"Focusing element: {selector}")
                locator = page.locator(selector)
                await locator.focus(timeout=timeout)

            click_lib.echo(f"Pressing key: {key}")
            await page.keyboard.press(key)

            click_lib.echo(f"Successfully pressed key: {key}")

        except Exception as e:
            click_lib.echo(f"Failed to press key: {e}", err=True)

if __name__ == "__main__":
    main()
