"""Check the docs navigation markup and CSS in headless Microsoft Edge.

Uses a local fixture extracted from the repository template, without Jekyll
or external theme CSS. This is a keyboard/accessibility-tree check, not an
NVDA or JAWS test.
"""

import json
from pathlib import Path
import re

from playwright.sync_api import sync_playwright


template = Path("docs/_layouts/default.html").read_text(encoding="utf-8")
navigation = re.search(r"<nav\b.*?</nav>", template, re.S).group()
navigation = re.sub(r"{{.*?}}", "/audit-link", navigation)
styles = re.search(r"<style>.*?</style>", template, re.S).group()
fixture = '<!doctype html><html lang="en"><title>Navigation audit</title>' + styles
fixture += '<body><a href="#main">Before menu</a>' + navigation
fixture += '<main id="main"><button>After menu</button></main></body></html>'
results = {}
with sync_playwright() as playwright:
    browser = playwright.chromium.launch(channel="msedge", headless=True)
    try:
        for width in (800, 375):
            page = browser.new_page(viewport={"width": width, "height": 800})
            page.set_content(fixture)
            focus = []
            for _ in range(10):
                page.keyboard.press("Tab")
                focus.append(page.evaluate("""() => ({
                    tag: document.activeElement.tagName,
                    text: document.activeElement.textContent.trim().slice(0, 80),
                    id: document.activeElement.id
                })"""))
            results[str(width)] = {
                "visible_navigation_links": page.locator("nav a:visible").count(),
                "navigation_accessibility_tree": page.locator("nav").aria_snapshot(),
                "tab_sequence": focus,
            }
            if width == 375:
                page.locator('#nav-trigger').focus()
                page.keyboard.press("Space")
                results[str(width)]["visible_links_after_space"] = page.locator("nav a:visible").count()
                results[str(width)]["checked_after_space"] = page.locator('#nav-trigger').is_checked()
                page.keyboard.press("Tab")
                results[str(width)]["first_link_after_open"] = page.evaluate("document.activeElement.textContent.trim()")
                page.keyboard.press("Shift+Tab")
                page.keyboard.press("Space")
                results[str(width)]["visible_links_after_close"] = page.locator("nav a:visible").count()
                page.keyboard.press("Tab")
                results[str(width)]["focus_after_close"] = page.evaluate("document.activeElement.textContent.trim()")
            page.close()
    finally:
        browser.close()
print(json.dumps(results, indent=2))
