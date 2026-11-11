import { test, expect } from "@playwright/test";
import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  readFileSync,
  rmSync,
  readdirSync,
  existsSync,
  symlinkSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawn, ChildProcess } from "node:child_process";
let home: string, link: string, server: ChildProcess;
function file(path: string, content: string) {
  const full = join(home, path);
  mkdirSync(resolve(full, ".."), { recursive: true });
  writeFileSync(full, content);
}
test.beforeAll(async () => {
  home = mkdtempSync(join(tmpdir(), "agent-desk-e2e-"));
  file(
    ".codex/config.toml",
    'model = "demo"\n[mcp_servers.docs]\nurl = "https://example.test/mcp"\n',
  );
  file(".codex/AGENTS.md", "# Instructions\n\nKeep changes focused.\n");
  file(
    ".agents/skills/review/SKILL.md",
    "---\nname: review\ndescription: Review changes\n---\n\nReview the diff.\n",
  );
  file(".claude/settings.json", "{}");
  file(".grok/config.toml", "");
  file(".config/opencode/opencode.json", "{}");
  file(".gemini/antigravity-cli/settings.json", "{}");
  file(
    ".codex/sessions/2026/01/01/demo.jsonl",
    JSON.stringify({
      type: "session_meta",
      payload: { id: "demo", cwd: home },
    }) +
      "\n" +
      JSON.stringify({
        type: "response_item",
        payload: {
          type: "message",
          role: "user",
          content: [{ type: "input_text", text: "Review the application" }],
        },
      }) +
      "\n" +
      JSON.stringify({
        type: "response_item",
        payload: {
          type: "message",
          role: "assistant",
          content: [
            { type: "reasoning", text: "Checking the build first." },
            { type: "output_text", text: "The application looks ready." },
          ],
        },
      }) +
      "\n",
  );
  server = spawn(resolve("../bin/agent-desk"), [
    "--home",
    home,
    "--port",
    "0",
    "--no-open",
  ]);
  link = await new Promise<string>((ok, no) => {
    let data = "";
    const timeout = setTimeout(
      () => no(new Error("Server startup timeout")),
      10000,
    );
    server.stdout!.on("data", (chunk) => {
      data += chunk.toString();
      const match = data.match(/http:\/\/localhost:\d+\/#token=[a-f0-9]+/);
      if (match) {
        clearTimeout(timeout);
        ok(match[0]);
      }
    });
    server.on("error", no);
    server.on("exit", (code) => {
      clearTimeout(timeout);
      no(new Error("Server exited " + code));
    });
  });
});
test.afterAll(async () => {
  if (server && server.exitCode === null) {
    server.kill();
    await new Promise((r) => server.once("exit", r));
  }
  if (home) rmSync(home, { recursive: true, force: true });
});
test("browse agents, review and save instructions, create a skill, read a session", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(link);
  await expect(page.getByRole("heading", { name: "Codex" })).toBeVisible();
  await page.screenshot({ path: "test-results/overview.png", fullPage: true });
  await expect(page.locator(".nav-caption")).toHaveText("Agents 5");
  for (const name of ["Claude Code", "OpenCode", "Grok", "Agy", "Codex"]) {
    await page.locator(".agent-nav").filter({ hasText: name }).click();
    await expect(page.getByRole("heading", { name, level: 1 })).toBeVisible();
  }
  await page.getByRole("tab", { name: "Extensions" }).click();
  await expect(
    page.locator(".extension-row strong").getByText("docs", { exact: true }),
  ).toBeVisible();
  await expect(
    page.locator(".extension-row strong").getByText("review", { exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Files" }).click();
  await page
    .locator(".file-row")
    .filter({ hasText: ".codex/AGENTS.md" })
    .click();
  await page
    .locator(".cm-content")
    .fill("# Updated instructions\n\nRun the focused checks.\n");
  await page.getByRole("button", { name: "Review changes" }).click();
  // The review marks removed and added lines.
  await expect(page.getByText("1 changed section")).toBeVisible();
  await expect(page.locator(".cm-deletedChunk")).toContainText(
    "Keep changes focused.",
  );
  await expect(page.locator(".cm-changedLine").first()).toBeVisible();
  await page.screenshot({ path: "test-results/review.png" });
  expect(readFileSync(join(home, ".codex/AGENTS.md"), "utf8")).toContain(
    "Keep changes focused",
  );
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(
    page.getByRole("button", { name: "Undo last save" }),
  ).toBeVisible();
  expect(readFileSync(join(home, ".codex/AGENTS.md"), "utf8")).toContain(
    "Run the focused checks",
  );
  expect(
    readdirSync(join(home, ".local/state/agent-desk/backups")).length,
  ).toBe(1);
  // Undo is still offered after closing and reopening the file.
  await page.getByRole("button", { name: "Close editor" }).click();
  await page
    .locator(".file-row")
    .filter({ hasText: ".codex/AGENTS.md" })
    .click();
  await page.getByRole("button", { name: "Undo last save" }).click();
  await expect(page.getByRole("heading", { name: "After undo" })).toBeVisible();
  await page.getByRole("button", { name: "Confirm undo" }).click();
  await expect(page.getByRole("status")).toContainText("Undone.");
  expect(readFileSync(join(home, ".codex/AGENTS.md"), "utf8")).toContain(
    "Keep changes focused",
  );
  await expect(
    page.getByRole("button", { name: "Undo last save" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Close editor" }).click();
  await page.getByRole("tab", { name: "Extensions" }).click();
  await page.getByRole("button", { name: "New skill" }).click();
  await page.getByLabel("Name", { exact: true }).fill("check-work");
  await page
    .getByLabel("Description", { exact: true })
    .fill("Use when checking a change");
  await page.getByRole("button", { name: "Create and edit" }).click();
  await expect(page.getByRole("dialog", { name: "File editor" })).toBeVisible();
  expect(
    readFileSync(join(home, ".codex/skills/check-work/SKILL.md"), "utf8"),
  ).toContain("Use when checking a change");
  await page.getByRole("button", { name: "Close editor" }).click();
  file(".codex/skills/check-work/notes.md", "Extra notes\n");
  await page
    .locator(".extension-row")
    .filter({ hasText: "check-work" })
    .click();
  let question = "";
  page.once("dialog", (d) => {
    question = d.message();
    d.accept();
  });
  await page.getByRole("button", { name: "Remove skill" }).click();
  await expect(page.getByRole("status")).toContainText("Skill removed.");
  expect(question).toContain("2 files");
  expect(existsSync(join(home, ".codex/skills/check-work"))).toBe(false);
  await page.getByRole("tab", { name: "Sessions" }).click();
  await page
    .locator(".session-row")
    .filter({ hasText: "Review the application" })
    .click();
  await expect(
    page.getByText("The application looks ready.", { exact: true }),
  ).toBeVisible();
  const thinking = page.locator("details.message.thinking");
  await expect(thinking).toHaveCount(1);
  await expect(page.getByText("Checking the build first.")).toBeHidden();
  await thinking.locator("summary").click();
  await expect(page.getByText("Checking the build first.")).toBeVisible();
  await page.getByLabel("Find in conversation").fill("looks ready");
  await expect(page.locator("article.message")).toHaveCount(1);
  expect(errors).toEqual([]);
});
test("undo removes a file created by the last save, even after a reload", async ({
  page,
}) => {
  await page.goto(link);
  await page.locator(".agent-nav").filter({ hasText: "Grok" }).click();
  await page.getByRole("tab", { name: "Files" }).click();
  const row = page.locator(".file-row").filter({ hasText: ".grok/AGENTS.md" });
  await row.click();
  await page.locator(".cm-content").fill("# Grok notes\n");
  await page.getByRole("button", { name: "Review changes" }).click();
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(page.getByRole("status")).toContainText("File created.");
  await page.reload();
  await page.locator(".agent-nav").filter({ hasText: "Grok" }).click();
  await page.getByRole("tab", { name: "Files" }).click();
  await row.click();
  await page.getByRole("button", { name: "Undo last save" }).click();
  await expect(page.getByText("The file will be removed")).toBeVisible();
  await page.getByRole("button", { name: "Confirm undo" }).click();
  await expect(page.getByRole("status")).toContainText("was removed");
  expect(existsSync(join(home, ".grok/AGENTS.md"))).toBe(false);
});
test("JSON settings changed by another program are merged on save", async ({
  page,
}) => {
  await page.goto(link);
  await page.locator(".agent-nav").filter({ hasText: "Claude Code" }).click();
  await page.getByRole("tab", { name: "Files" }).click();
  await page
    .locator(".file-row")
    .filter({ hasText: ".claude/settings.json" })
    .click();
  await expect(page.locator(".cm-content")).toBeVisible();
  writeFileSync(
    join(home, ".claude/settings.json"),
    '{\n  "cleanupPeriodDays": 30\n}\n',
  );
  await page.locator(".cm-content").fill('{"theme": "light"}');
  await page.getByRole("button", { name: "Review changes" }).click();
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(page.getByRole("status")).toContainText("combined");
  const saved = JSON.parse(
    readFileSync(join(home, ".claude/settings.json"), "utf8"),
  );
  expect(saved).toEqual({ cleanupPeriodDays: 30, theme: "light" });
});
test("files reached through a linked folder can be edited", async ({
  page,
}) => {
  file("dotfiles/grok/config.toml", 'model = "a"\n');
  rmSync(join(home, ".grok"), { recursive: true, force: true });
  symlinkSync(join(home, "dotfiles/grok"), join(home, ".grok"));
  await page.goto(link);
  await page.locator(".agent-nav").filter({ hasText: "Grok" }).click();
  await page.getByRole("tab", { name: "Files" }).click();
  await page
    .locator(".file-row")
    .filter({ hasText: ".grok/config.toml" })
    .click();
  await expect(page.getByText("This path is a link.")).toBeVisible();
  await page.locator(".cm-content").fill('model = "b"\n');
  await page.getByRole("button", { name: "Review changes" }).click();
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(page.getByRole("status")).toContainText("Saved.");
  expect(readFileSync(join(home, "dotfiles/grok/config.toml"), "utf8")).toBe(
    'model = "b"\n',
  );
});
test("long conversations load in pages and huge messages are shortened", async ({
  page,
}) => {
  const line = (role: string, text: string) =>
    JSON.stringify({
      type: "response_item",
      payload: {
        type: "message",
        role,
        content: [
          { type: role === "user" ? "input_text" : "output_text", text },
        ],
      },
    }) + "\n";
  let content =
    JSON.stringify({
      type: "session_meta",
      payload: { id: "long", cwd: home },
    }) +
    "\n" +
    line("user", "Long conversation start");
  for (let i = 1; i < 249; i++)
    content += line("assistant", `Reply number ${i}`);
  content += line("assistant", "x".repeat(30000));
  file(".codex/sessions/2026/01/02/long.jsonl", content);
  await page.goto(link);
  await page.getByRole("tab", { name: "Sessions" }).click();
  await page
    .locator(".session-row")
    .filter({ hasText: "Long conversation start" })
    .click();
  await expect(page.getByText("Showing 200 of 250 messages")).toBeVisible();
  await expect(page.locator("article.message")).toHaveCount(200);
  await page.getByRole("button", { name: "Show more" }).click();
  await expect(page.locator("article.message")).toHaveCount(250);
  await expect(page.getByRole("button", { name: "Show more" })).toHaveCount(0);
  const full = page.getByRole("button", { name: /Show full message/ });
  await expect(full).toContainText("30,000 characters");
  await full.click();
  await expect(full).toHaveCount(0);
  await page.getByLabel("Find in conversation").fill("Reply number 123");
  await expect(
    page.getByText("Showing 1 of 1 matching messages"),
  ).toBeVisible();
});
test("invalid configuration and stale revisions cannot overwrite files", async ({
  page,
}) => {
  await page.goto(link);
  await page.getByRole("tab", { name: "Files" }).click();
  await page.locator(".file-row").filter({ hasText: "config.toml" }).click();
  // TOML gets syntax colouring.
  await expect(page.locator(".cm-content span").first()).toBeVisible();
  await page.locator(".cm-content").fill("model = [");
  await page.getByRole("button", { name: "Review changes" }).click();
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "invalid configuration",
  );
  expect(readFileSync(join(home, ".codex/config.toml"), "utf8")).toContain(
    'model = "demo"',
  );
  await page.getByRole("button", { name: "Back to editing" }).click();
  await page.locator(".cm-content").fill('model = "changed"\n');
  writeFileSync(join(home, ".codex/config.toml"), 'model = "external"\n');
  await page.getByRole("button", { name: "Review changes" }).click();
  await page.getByRole("button", { name: "Confirm save" }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText(
    "file changed",
  );
  expect(readFileSync(join(home, ".codex/config.toml"), "utf8")).toContain(
    "external",
  );
});
test("small screens keep navigation and content accessible", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(link);
  await expect(page.getByRole("heading", { name: "Codex" })).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await page.screenshot({ path: "test-results/mobile.png", fullPage: true });
});
