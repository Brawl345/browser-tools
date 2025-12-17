import asyncio
import click
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page, get_context

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
def main(port):
    """List all cookies from the current page in an existing Chrome instance."""
    asyncio.run(list_cookies(port))

async def list_cookies(port):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            context = await get_context(browser)
            if not context:
                return

            current_url = page.url
            cookies = await context.cookies(current_url)

            if not cookies:
                click.echo("No cookies found")
                return

            for cookie in cookies:
                click.echo(f"Name: {cookie['name']}")
                click.echo(f"  Value: {cookie['value']}")
                click.echo(f"  Domain: {cookie['domain']}")
                click.echo(f"  Path: {cookie['path']}")
                click.echo(f"  Secure: {cookie.get('secure', False)}")
                click.echo(f"  HttpOnly: {cookie.get('httpOnly', False)}")
                click.echo(f"  SameSite: {cookie.get('sameSite', 'None')}")
                click.echo()
        except Exception as e:
            click.echo(f"Failed to connect or list cookies: {e}", err=True)

if __name__ == "__main__":
    main()
