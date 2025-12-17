# Browser Tools Skill

This folder contains the "browser-tools" skill for Claude, which allows Claude to use the Chrome browser to perform various actions. The Python scripts are located in the `scripts` folder and use Playwright. Shared logic is in `browser_utils.py`. The scripts should always follow the same structure.

`SKILL.md` contains a short description of each tool — this should be concise, as it is loaded into Claude’s context window. A full description can be found in `REFERENCE.md`. A short description of each script must be written in `README.md`.

Scripts must not block execution and MUST return a result immediately.

## Development

To check python types, run:

```bash
uv run ty check scripts/
```

To lint and format code, run:

```bash
uv run ruff check scripts/
# Add --fix to automatically fix safe issues
```

## References

* [Skills documentation](https://docs.claude.com/en/docs/claude-code/skills)
* [Playwright Python docs](https://playwright.dev/python/docs/api/class-playwright)
