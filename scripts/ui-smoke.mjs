import { existsSync } from "node:fs";

const { chromium } = await import(new URL("../frontend/node_modules/playwright/index.mjs", import.meta.url));
const url = process.env.HOSTSWITCH_UI_URL ?? "http://localhost:5174";
const screenshotPath = process.env.HOSTSWITCH_UI_SCREENSHOT ?? "dist/ui-smoke.png";
const chromePath = process.env.PLAYWRIGHT_CHROME_PATH ?? findChromePath();

if (!chromePath) {
  throw new Error("Chrome or Edge was not found. Set PLAYWRIGHT_CHROME_PATH to a Chromium-based browser executable.");
}

const browser = await chromium.launch({
  headless: true,
  executablePath: chromePath,
});
const page = await browser.newPage({ viewport: { width: 1366, height: 900 } });
const errors = [];

page.on("console", (message) => {
  const text = message.text();
  if (message.type() === "error" && !text.includes("favicon") && !text.includes("status of 404")) {
    errors.push(text);
  }
});
page.on("pageerror", (error) => errors.push(error.message));

await page.goto(url, { waitUntil: "networkidle" });
for (const text of ["voxera.local", "whisper.local", "grafana.local", "Developer Tools", "Monitoring"]) {
  await page.getByText(text).first().waitFor({ state: "visible", timeout: 10000 });
}

for (const environment of ["STAGING", "PROD"]) {
  await page.getByRole("button", { name: environment }).click();
  await page.getByText("No hosts yet. Use Import Current to load entries into this environment.").waitFor({
    state: "visible",
    timeout: 10000,
  });
  await page.getByText("AI Stack").waitFor({ state: "visible", timeout: 10000 });
  await page.getByText("Monitoring").waitFor({ state: "visible", timeout: 10000 });
  await page.getByText("Tools").waitFor({ state: "visible", timeout: 10000 });
}

await page.getByRole("button", { name: "DEV" }).click();
await page.getByText("voxera.local").first().waitFor({ state: "visible", timeout: 10000 });

for (const navItem of ["Environments", "Docker Discovery", "Backups", "Settings", "API", "Dashboard"]) {
  await page.getByRole("button", { name: navItem }).click();
  await page.getByRole("heading", { name: navItem, exact: true }).waitFor({ state: "visible", timeout: 10000 });
}

const expectedContent = [
  ["Environments", "Hosts In View"],
  ["Docker Discovery", "Discovered Services"],
  ["Backups", "Backup History"],
  ["Settings", "Hosts File Safety"],
  ["API", "REST API"],
  ["Dashboard", "Hosts"],
];
for (const [navItem, content] of expectedContent) {
  await page.getByRole("button", { name: navItem }).click();
  await page.getByText(content).first().waitFor({ state: "visible", timeout: 10000 });
}

await page.screenshot({ path: screenshotPath, fullPage: true });
await browser.close();

if (errors.length > 0) {
  throw new Error(`UI smoke test saw browser errors:\n${errors.join("\n")}`);
}

console.log(`UI smoke passed at ${url}. Screenshot: ${screenshotPath}`);

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
