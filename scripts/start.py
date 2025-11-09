#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright",
# ]
# ///

import asyncio
import os
import platform
import shutil
import subprocess
import time
import click
from pathlib import Path
from playwright.async_api import async_playwright

@click.command()
@click.option(
    "--browser",
    type=click.Choice(["chrome-stable", "chrome-beta", "chrome-dev", "chrome-canary"]),
    default="chrome-stable",
    help="Chrome browser variant to launch (default: chrome-stable)"
)
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--path",
    type=click.Path(exists=True),
    help="Path to custom Chrome/Chromium executable"
)
def main(browser, port, path):
    """Launch Chrome with remote debugging and verify connection."""
    if path and browser != "chrome-stable":
        click.echo("Error: Cannot specify both --browser and --path", err=True)
        return
    asyncio.run(launch_and_verify(browser, port, path))

def get_browser_config():
    system = platform.system()

    if system == "Darwin":
        return {
            "chrome-stable": {
                "app_name": "Google Chrome",
                "executable": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
                "process_name": "Google Chrome"
            },
            "chrome-beta": {
                "app_name": "Google Chrome Beta",
                "executable": "/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
                "process_name": "Google Chrome Beta"
            },
            "chrome-dev": {
                "app_name": "Google Chrome Dev",
                "executable": "/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev",
                "process_name": "Google Chrome Dev"
            },
            "chrome-canary": {
                "app_name": "Google Chrome Canary",
                "executable": "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
                "process_name": "Google Chrome Canary"
            }
        }
    elif system == "Windows":
        localappdata = os.environ.get("LOCALAPPDATA", "")
        programfiles = os.environ.get("PROGRAMFILES", "C:\\Program Files")
        programfiles_x86 = os.environ.get("PROGRAMFILES(X86)", "C:\\Program Files (x86)")

        return {
            "chrome-stable": {
                "app_name": "Google Chrome",
                "executable": f"{programfiles}\\Google\\Chrome\\Application\\chrome.exe",
                "process_name": "chrome.exe"
            },
            "chrome-beta": {
                "app_name": "Google Chrome Beta",
                "executable": f"{programfiles}\\Google\\Chrome Beta\\Application\\chrome.exe",
                "process_name": "chrome.exe"
            },
            "chrome-dev": {
                "app_name": "Google Chrome Dev",
                "executable": f"{localappdata}\\Google\\Chrome Dev\\Application\\chrome.exe",
                "process_name": "chrome.exe"
            },
            "chrome-canary": {
                "app_name": "Google Chrome Canary",
                "executable": f"{localappdata}\\Google\\Chrome SxS\\Application\\chrome.exe",
                "process_name": "chrome.exe"
            }
        }
    else:
        return {
            "chrome-stable": {
                "app_name": "Google Chrome",
                "executable": shutil.which("google-chrome") or shutil.which("chrome") or "/usr/bin/google-chrome",
                "process_name": "chrome"
            },
            "chrome-beta": {
                "app_name": "Google Chrome Beta",
                "executable": shutil.which("google-chrome-beta") or "/usr/bin/google-chrome-beta",
                "process_name": "chrome"
            },
            "chrome-dev": {
                "app_name": "Google Chrome Dev",
                "executable": shutil.which("google-chrome-unstable") or "/usr/bin/google-chrome-unstable",
                "process_name": "chrome"
            },
            "chrome-canary": {
                "app_name": "Google Chrome Canary",
                "executable": shutil.which("google-chrome-canary") or "/usr/bin/google-chrome-canary",
                "process_name": "chrome"
            }
        }

def kill_browser(process_name):
    system = platform.system()
    try:
        if system == "Windows":
            subprocess.run(["taskkill", "/F", "/IM", process_name],
                         check=False, capture_output=True)
        else:
            subprocess.run(["pkill", "-f", process_name],
                         check=False, capture_output=True)
    except Exception:
        pass

def get_user_data_dir(browser_variant):
    system = platform.system()

    if system == "Windows":
        base_dir = Path(os.environ.get("LOCALAPPDATA", "")) / "claude-browser-tools"
    elif system == "Darwin":
        base_dir = Path.home() / ".cache" / "claude-browser-tools"
    else:
        base_dir = Path.home() / ".cache" / "claude-browser-tools"

    return str(base_dir / browser_variant)

async def launch_and_verify(browser_variant, port, custom_path=None):
    try:
        if custom_path:
            executable = custom_path
            app_name = Path(custom_path).name
            process_name = Path(custom_path).name
        else:
            browser_config = get_browser_config()
            config = browser_config[browser_variant]
            executable = config["executable"]
            app_name = config["app_name"]
            process_name = config["process_name"]

            if not Path(executable).exists() and not shutil.which(executable):
                click.echo(f"{app_name} not found at {executable}", err=True)
                click.echo(f"Please install {app_name} or use a different browser variant.", err=True)
                return

        kill_browser(process_name)
        await asyncio.sleep(1)

        user_data_dir = get_user_data_dir(browser_variant if not custom_path else "custom")
        Path(user_data_dir).mkdir(parents=True, exist_ok=True)

        subprocess.Popen([
            executable,
            f"--remote-debugging-port={port}",
            f"--user-data-dir={user_data_dir}",
            "--no-first-run",
            "--no-default-browser-check"
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        start_new_session=True if platform.system() != "Windows" else False
        )

        click.echo(f"Starting {app_name} with remote debugging on port {port}")

        async with async_playwright() as p:
            for attempt in range(5):
                try:
                    await asyncio.sleep(1 + attempt * 0.5)

                    browser = await p.chromium.connect_over_cdp(f"http://localhost:{port}")

                    await browser.close()
                    click.echo(f"{config['app_name']} successfully started")
                    break

                except Exception as e:
                    if attempt == 4:
                        click.echo(f"Failed to connect after 5 attempts: {e}", err=True)
                        click.echo("Browser may have failed to start properly", err=True)
                    else:
                        click.echo(f"Attempt {attempt + 1} failed, retrying...", err=True)

    except subprocess.CalledProcessError as e:
        click.echo(f"Failed to launch browser: {e}", err=True)
    except FileNotFoundError as e:
        click.echo(f"Browser executable not found: {e}", err=True)

if __name__ == "__main__":
    main()