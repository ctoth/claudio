# NVDA navigation verification

Tested September 12, 2026, with NVDA 2026.1.1 and Firefox on Windows, using the screen-reader MCP server.

The test page was extracted from `docs/_layouts/default.html`, with local placeholder content and the template's own CSS. It did not include the external Jekyll theme. Firefox reported an actual viewport width of 500 pixels after a request for 375 pixels. Both widths match the template's mobile breakpoint. This verifies the navigation component, not a complete built-site accessibility audit.

Native Windows keyboard input confirmed these transitions:

- Tab reached the menu control. NVDA said, "Primary navigation landmark Navigation menu check box not checked."
- Space opened the menu. NVDA said, "checked." DOM inspection found seven visible navigation links.
- Tab reached the first link. NVDA said, "Home same page link."
- Returning to the control and pressing Space closed the menu. NVDA said, "not checked." DOM inspection found zero visible navigation links.
- Tab after closing reached the main content link. NVDA said, "Content main landmark Content link same page link." Hidden navigation links were skipped.
- The skip link was reachable and announced as "Skip to content same page link." Its destination was not separately verified through NVDA.

The "same page link" wording comes from placeholder URLs in the fixture, not from the deployed site.

The control remains a native checkbox with a checked state. It does not announce an expanded state. This preserves the existing CSS-only interaction while making it reachable, named, and operable with a keyboard.

Only the relevant speech above is retained. The raw live logs also contained unrelated desktop announcements, so they are not copied into the repository.

The successful sequence used native Windows keyboard input. Browser-protocol key presses did not reliably exercise NVDA browse-cursor behavior.
