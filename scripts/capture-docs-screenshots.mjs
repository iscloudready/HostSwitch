import { existsSync, mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";

const { chromium } = await import(new URL("../frontend/node_modules/playwright/index.mjs", import.meta.url));

const uiUrl = process.env.HOSTSWITCH_UI_URL ?? "http://localhost:5175";
const apiUrl = process.env.HOSTSWITCH_API_URL ?? "http://localhost:18082";
const outDir = resolve(process.env.HOSTSWITCH_DOCS_IMAGE_DIR ?? "docs/images");
const chromePath = process.env.PLAYWRIGHT_CHROME_PATH ?? findChromePath();

if (!chromePath) {
  throw new Error("Chrome or Edge was not found. Set PLAYWRIGHT_CHROME_PATH to a Chromium-based browser executable.");
}

mkdirSync(outDir, { recursive: true });

await waitFor(`${apiUrl}/api/health`);
await fetch(`${apiUrl}/api/hosts/import-system`, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ scope: "all" }),
});
await waitFor(uiUrl);

const browser = await chromium.launch({
  headless: true,
  executablePath: chromePath,
});

try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 980 }, deviceScaleFactor: 1 });
  const errors = [];
  page.on("console", (message) => {
    const text = message.text();
    if (message.type() === "error" && !text.includes("status of 404")) {
      errors.push(text);
    }
  });
  page.on("pageerror", (error) => errors.push(error.message));

  await page.goto(uiUrl, { waitUntil: "networkidle" });
  await page.getByText(/local/i).first().waitFor({ state: "visible", timeout: 15000 });

  await screenshot(page, "Dashboard", "dashboard-hero.png", true);
  await screenshot(page, "Dashboard", "dashboard.png", false);
  await screenshot(page, "Environments", "environments.png", false);
  await screenshot(page, "Docker Discovery", "docker-discovery.png", false);
  await screenshot(page, "API", "api.png", false);

  if (errors.length > 0) {
    throw new Error(`Browser errors while capturing docs screenshots:\n${errors.join("\n")}`);
  }
} finally {
  await browser.close();
}

console.log(`Documentation screenshots written to ${outDir}`);

async function screenshot(page, navName, fileName, fullPage) {
  await page.getByRole("button", { name: navName }).click();
  await page.locator("h1").filter({ hasText: navName }).waitFor({ state: "visible", timeout: 10000 });
  const path = resolve(outDir, fileName);
  mkdirSync(dirname(path), { recursive: true });
  await page.screenshot({ path, fullPage });
}

async function waitFor(url) {
  const deadline = Date.now() + 60_000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
      lastError = new Error(`${url} returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolveWait) => setTimeout(resolveWait, 1000));
  }
  throw lastError ?? new Error(`Timed out waiting for ${url}`);
}

function findChromePath() {
  return [
    "C:/Program Files/Google/Chrome/Application/chrome.exe",
    "C:/Program Files (x86)/Google/Chrome/Application/chrome.exe",
    "C:/Program Files/Microsoft/Edge/Application/msedge.exe",
    "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
  ].find((candidate) => existsSync(candidate));
}
