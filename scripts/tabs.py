import asyncio
import click
from playwright.async_api import async_playwright
from browser_utils import connect_to_browser, get_context

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--switch",
    type=int,
    help="Switch to tab by index (0-based)"
)
@click.option(
    "--close",
    type=int,
    help="Close tab by index (0-based)"
)
def main(port, switch, close):
    """List all open tabs, switch to a specific tab, or close a tab by index."""
    asyncio.run(manage_tabs(port, switch, close))

async def manage_tabs(port, switch_index, close_index):
    async with async_playwright():
        try:
            browser = await connect_to_browser(port)
            if not browser:
                return

            context = await get_context(browser)
            if not context:
                return

            pages = context.pages
            if not pages:
                click.echo("No tabs found", err=True)
                return

            if switch_index is not None and close_index is not None:
                click.echo("Cannot use --switch and --close together", err=True)
                return

            if switch_index is not None:
                if switch_index < 0 or switch_index >= len(pages):
                    click.echo(f"Invalid tab index: {switch_index}. Valid range: 0-{len(pages)-1}", err=True)
                    return

                page = pages[switch_index]
                await page.bring_to_front()
                click.echo(f"Switched to tab {switch_index}: {page.url}")
            elif close_index is not None:
                if close_index < 0 or close_index >= len(pages):
                    click.echo(f"Invalid tab index: {close_index}. Valid range: 0-{len(pages)-1}", err=True)
                    return

                page = pages[close_index]
                url = page.url
                await page.close()
                click.echo(f"Closed tab {close_index}: {url}")
            else:
                click.echo(f"Found {len(pages)} tab(s):")
                for idx, page in enumerate(pages):
                    title = await page.title()
                    url = page.url
                    click.echo(f"  [{idx}] {title or '(no title)'}")
                    click.echo(f"      {url}")

        except Exception as e:
            click.echo(f"Failed to manage tabs: {e}", err=True)

if __name__ == "__main__":
    main()
