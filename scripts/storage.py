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
from browser_utils import get_browser_and_page

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--local",
    is_flag=True,
    help="Show localStorage only"
)
@click.option(
    "--session",
    is_flag=True,
    help="Show sessionStorage only"
)
def main(port, local, session):
    """List localStorage and/or sessionStorage from an existing Chrome instance.

    By default, shows both localStorage and sessionStorage for the current page.
    """
    if local and not session:
        storage_type = "local"
    elif session and not local:
        storage_type = "session"
    else:
        storage_type = "all"

    asyncio.run(list_storage(port, storage_type))

async def list_storage(port, storage_type):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            if storage_type in ["local", "all"]:
                local_storage = await page.evaluate("""
                    () => {
                        const items = {};
                        for (let i = 0; i < localStorage.length; i++) {
                            const key = localStorage.key(i);
                            items[key] = localStorage.getItem(key);
                        }
                        return items;
                    }
                """)

                click.echo("=== localStorage ===")
                if local_storage:
                    for key, value in local_storage.items():
                        click.echo(f"{key}: {value}")
                else:
                    click.echo("(empty)")
                click.echo()

            if storage_type in ["session", "all"]:
                session_storage = await page.evaluate("""
                    () => {
                        const items = {};
                        for (let i = 0; i < sessionStorage.length; i++) {
                            const key = sessionStorage.key(i);
                            items[key] = sessionStorage.getItem(key);
                        }
                        return items;
                    }
                """)

                click.echo("=== sessionStorage ===")
                if session_storage:
                    for key, value in session_storage.items():
                        click.echo(f"{key}: {value}")
                else:
                    click.echo("(empty)")

        except Exception as e:
            click.echo(f"Failed to read storage: {e}", err=True)

if __name__ == "__main__":
    main()
