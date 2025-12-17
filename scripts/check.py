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
    "--uncheck",
    is_flag=True,
    help="Uncheck the checkbox (only for checkboxes)"
)
@click_lib.option(
    "--force",
    is_flag=True,
    help="Force check even if element is not visible or enabled"
)
def main(selector, port, timeout, uncheck, force):
    """Check/uncheck a checkbox or select a radio button using a CSS selector.

    Example:
      check.py "input#accept-terms"
      check.py "input[name='newsletter']" --uncheck
      check.py "input[type='radio'][value='option1']"
      check.py "#hidden-checkbox" --force
    """
    asyncio.run(check_element(selector, port, timeout, uncheck, force))

async def check_element(selector, port, timeout, uncheck, force):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for element: {selector}")

            locator = page.locator(selector)

            if uncheck:
                await locator.uncheck(force=force, timeout=timeout)
                click_lib.echo(f"Successfully unchecked: {selector}")
            else:
                await locator.check(force=force, timeout=timeout)
                click_lib.echo(f"Successfully checked: {selector}")

        except Exception as e:
            click_lib.echo(f"Failed to check element: {e}", err=True)

if __name__ == "__main__":
    main()
