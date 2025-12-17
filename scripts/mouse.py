import asyncio
import click as click_lib
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click_lib.command()
@click_lib.argument("action", type=click_lib.Choice(["click", "dblclick", "hover", "right-click", "drag"]))
@click_lib.argument("selector")
@click_lib.option(
    "--to",
    help="Target selector for drag action"
)
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
    help="Force action even if element is not visible or enabled"
)
@click_lib.option(
    "--delay",
    type=int,
    help="Delay between mousedown and mouseup in milliseconds"
)
def main(action, selector, to, port, timeout, force, delay):
    """Perform mouse actions on elements.

    Example:
      mouse.py click "button#submit"
      mouse.py dblclick ".item"
      mouse.py hover "nav .menu-item"
      mouse.py right-click ".context-menu-trigger"
      mouse.py drag ".draggable" --to ".drop-zone"
    """
    asyncio.run(perform_mouse_action(action, selector, to, port, timeout, force, delay))

async def perform_mouse_action(action, selector, to, port, timeout, force, delay):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for element: {selector}")

            locator = page.locator(selector)

            kwargs = {"timeout": timeout}
            if force:
                kwargs["force"] = True
            if delay:
                kwargs["delay"] = delay

            if action == "click":
                await locator.click(**kwargs)
                click_lib.echo(f"Successfully clicked: {selector}")

            elif action == "dblclick":
                await locator.dblclick(**kwargs)
                click_lib.echo(f"Successfully double-clicked: {selector}")

            elif action == "hover":
                await locator.hover(timeout=timeout, force=force if force else None)
                click_lib.echo(f"Successfully hovered over: {selector}")

            elif action == "right-click":
                await locator.click(button="right", **kwargs)
                click_lib.echo(f"Successfully right-clicked: {selector}")

            elif action == "drag":
                if not to:
                    click_lib.echo("Error: --to selector is required for drag action", err=True)
                    return

                click_lib.echo(f"Looking for target element: {to}")
                target = page.locator(to)

                await locator.drag_to(target, timeout=timeout, force=force if force else None)
                click_lib.echo(f"Successfully dragged {selector} to {to}")

        except Exception as e:
            click_lib.echo(f"Failed to perform {action}: {e}", err=True)

if __name__ == "__main__":
    main()
