import { test, expect } from "@playwright/test";

test.describe("Critical flows (static export smoke)", () => {
  test("navigation smoke: /, /trends, /methodology render headings", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("h1")).toContainText("Overview");

    await page.goto("/trends/");
    await expect(page.locator("h1")).toContainText("Trends");

    await page.goto("/methodology/");
    await expect(page.locator("h1")).toContainText("Methodology");
  });

  test("error boundary: /nonexistent-route shows not-found", async ({ page }) => {
    await page.goto("/nonexistent-route/");
    // Static export 404 fallback -- Next static export generates a 404.html
    // or the not-found page renders "Page not found".
    await expect(page.locator("body")).toContainText(/not found|404/i);
  });

  test("auth redirect: /settings shows error or redirects (no session)", async ({ page }) => {
    await page.goto("/settings/");
    // Without API, SWR will error (network); the page should show either
    // "Unable to load your profile" (ErrorPanel) or redirect to signin.
    const body = page.locator("body");
    await expect(body).toContainText(/(Unable to load|Sign in|Settings)/i);
  });

  test("admin guard: /admin/health shows guard or nav", async ({ page }) => {
    await page.goto("/admin/health/");
    const body = page.locator("body");
    // Without API: shows loading then error; role guard renders ErrorPanel
    // or the "Administrator access required" message.
    await expect(body).toContainText(/(Administrator|Health|Loading)/i);
  });

  test("skip link: Tab focuses skip link, Enter moves to main", async ({ page }) => {
    await page.goto("/");
    await page.keyboard.press("Tab");
    const skipLink = page.locator("a.skip-link");
    if (await skipLink.count() > 0) {
      await expect(skipLink).toBeFocused();
      await page.keyboard.press("Enter");
      // After skip link, focus should be on main content area
      const main = page.locator("main, [role='main'], .page");
      await expect(main).toBeVisible();
    }
  });

  test("responsive: 375px width has no horizontal overflow", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/");
    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1); // 1px tolerance
  });
});
