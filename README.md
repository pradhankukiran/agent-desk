# Agent Desk

**A local workspace for your coding agents.**

Browse and edit the configuration behind Codex, Claude Code, OpenCode, Grok, and Agy from one browser interface. Inspect extensions, maintain instruction files, and explore conversation history while keeping each agent's native files as the source of truth.

Agent Desk runs as a Go executable with an embedded React interface built on GitHub's [Primer](https://primer.style) design system. It binds to localhost, opens your browser, and follows your system's light or dark mode. No cloud account, hosted backend, or agent API key is required.

**Status:** Early release, developed and tested on Linux through WSL. Other platforms and agent versions have not been verified.

[Getting started](#getting-started) · [Features](#features) · [Agent support](#agent-support) · [Safety and privacy](#safety-and-privacy) · [Development](#development) · [Limitations](#limitations)

## Getting started

### Prerequisites

To build from source:

- Go **1.22.2 or newer**.
- Node.js **20.19+ or 22.12+**, with npm. Node.js 26 was used during development.
- Git and Make.

Node.js is a build dependency. The resulting executable includes the browser interface and does not require Node.js at runtime.

### Build and launch

```sh
git clone https://github.com/pradhankukiran/agent-desk.git
cd agent-desk
make build
./bin/agent-desk
```

Agent Desk starts at `http://localhost:7437` and attempts to open your browser. If that port is in use, it chooses a free port and prints the new address; pass `--port` to require a specific port. If automatic opening is unavailable, use the full URL printed in the terminal. Press **Ctrl+C** to stop the server.

The printed URL includes an access token in its fragment. The interface removes that fragment after loading and keeps the token in the current browser tab's session storage. Use the newly printed URL after restarting the server.

### Optional local installation

Place the executable in a directory on your `PATH`:

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 ./bin/agent-desk "$HOME/.local/bin/agent-desk"
agent-desk
```

The final command assumes `~/.local/bin` is already on your `PATH`.

## Features

| Area | Available now |
| --- | --- |
| Agents | Switch agents from the header bar. Each shows whether it is installed on `PATH` and how many extensions were found. |
| Project scope | Add a project's local files to the user configuration. The project menu suggests folders your agents have worked in and folders under `~/projects`. |
| Files | Settings and instruction files in one list, with path, scope, size, and read-only status. Edit JSON, JSONC, TOML, and Markdown, including `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md`. |
| Extensions | MCP server declarations, skills, plugins, hooks, agent definitions, commands, and rules in supported locations, filterable by type. |
| Skills | Create a skill, edit its `SKILL.md`, or remove a skill together with its folder. |
| Editing | Files open in a side panel beside the list. Changes are validated and shown as a line-by-line comparison before saving. |
| Sessions | Browse conversations by title and folder, filter messages across the whole conversation, load long conversations in pages, and copy message text. |
| Change protection | Revision checks, merging of outside JSON changes, automatic backups, permission preservation, and reviewable undo of the last save. |
| Keyboard | <kbd>/</kbd> focuses search; <kbd>Esc</kbd> closes the editor or a dialog. |

MCP servers, plugins, and hooks can be configured by editing their source files. Agent Desk does not connect to MCP servers, run conversations, or install plugins.

## Agent support

The adapters inspect known local paths and storage formats. The paths below are the primary locations; project and shared-skill discovery add further sources where supported.

| Agent | Primary configuration | Session reader |
| --- | --- | --- |
| Codex | `~/.codex/config.toml` | JSONL sessions and archived sessions. |
| Claude Code | `~/.claude/settings.json`, `~/.claude.json` | Project JSONL conversations. |
| OpenCode | `~/.config/opencode/opencode.json` and `.jsonc` | SQLite history using both supported session schemas. |
| Grok | `~/.grok/config.toml` | JSONL chat history. |
| Agy | `~/.gemini/antigravity-cli/settings.json`, `~/.gemini/config/` | Conversation metadata and indexed text excerpts. |

### Understanding the inventory

- **Present** means a definition was found on disk. It does not prove the agent has loaded it.
- **Configured** means an entry exists in a recognized configuration map. It does not indicate connection health.
- **Disabled** reflects an explicit disable value recognized by the adapter.
- **Not created** identifies a known configuration or instruction file that can be created through the editor.

Shared skills may appear under multiple agents. Bundled, installed-plugin, and managed files are generally view-only. Supported user and project files can be edited, including through symbolic links whose real location is inside your home folder, such as a linked dotfiles folder.

### Agy conversation history

Agy stores full conversation steps as binary payloads. Agent Desk reads the local search index instead. These excerpts may omit content and group user and assistant text by index chunk. The interface labels them as excerpts rather than a complete, ordered transcript.

## Everyday use

1. Select an agent in the header bar.
2. Keep **No project** selected, or choose a project to include its local files. Choose **Other folder…** to enter a folder that is not listed.
3. Open **Files** or **Extensions** and select an item to open its source file in the side panel.
4. Make changes, select **Review changes** to see the comparison, then **Confirm save**.
5. Open **Sessions** to browse history and filter the messages in a selected conversation.

Use the reload button in the header to rescan configuration and session history. An already-open editor keeps its snapshot; close and reopen it to load outside changes. Saving a JSON file that another program changed in the meantime merges both sets of changes when they touch different settings.

### Command-line options

| Option | Default | Purpose |
| --- | --- | --- |
| `--port` | `7437` | Require a specific port. Set `0` to select an available port. Without this option, a busy default port falls back to a free one. |
| `--no-open` | `false` | Print the URL without opening a browser. |
| `--home` | Current user's home | Inspect agent files under a different home directory. Use an absolute path. |

```sh
./bin/agent-desk --port 0
./bin/agent-desk --no-open
./bin/agent-desk --home /absolute/path/to/demo-home --no-open
```

Project selection is restricted to directories inside the selected home. Agent executable detection uses the process's `PATH`, including when `--home` points to a fixture directory.

## Safety and privacy

Agent Desk can read conversation text and edit real configuration files. Treat its browser tab and access URL as local administrative access.

- The server listens on **127.0.0.1**.
- Every API request requires a random token generated at startup.
- API requests from foreign browser origins are rejected; accepted hostnames are restricted to localhost.
- File operations use catalog identifiers rather than arbitrary file paths.
- Writes outside the selected home are rejected. Edits through a symbolic link are saved to its real location inside home and leave the link in place; files that are themselves links cannot be removed here.
- Session databases are opened in read-only mode.
- The app does not intentionally select credential-only files such as `auth.json`. Configuration and session text can still contain credentials or other private information.
- The app serves local assets and does not send configuration or conversation data to an external service.

Do not expose the server through a public proxy or share its access URL. This is a local, single-user application.

### Saves and backups

Saving validates configuration syntax, checks the opened revision, preserves existing file permissions, and replaces the target through a temporary file and rename. If another program changed a plain JSON configuration file after you opened it, Agent Desk merges your edits with those changes when they touch different settings, keeping the file's key order. If both changed the same setting, the save is refused and names the setting. Removing an editable skill or instruction file also creates a backup.

Backups are private JSON files stored under:

```text
~/.local/state/agent-desk/backups/
```

Each file backup records the original `path`, `content`, and save time. Removing a skill copies its whole folder here. **Undo last save** shows the previous content for review and remains available until the file changes elsewhere or Agent Desk restarts. Older backups can be restored manually using their recorded path and content.

For each file, Agent Desk keeps its 20 newest backups and every backup from the last 30 days. Older backups beyond that are deleted when a new backup is made.

Backups may contain sensitive values. Revision checks reduce accidental overwrites, but they are not a locking protocol shared with running agent processes. Validation checks syntax, not every agent-specific setting or its operational effect.

## Architecture

```text
Browser: React + TypeScript + Primer + CodeMirror
                   │
              Local HTTP API
                   │
             Go application
                   │
    Agent configuration and session readers
                   │
      Native files / read-only SQLite
```

The app does not create a separate configuration database. Existing agent files remain authoritative; application-owned state consists of backups on disk and, while the app runs, the undo record and caches of the file inventory and session list.

```text
main.go                        CLI, embedded assets, browser launch, port fallback
internal/desk/
  app.go, models.go            Server state and API data types
  agents.go                    Agent-specific discovery locations
  catalog.go                   File and extension inventory, lookup cache
  config.go                    JSON / JSONC / TOML parsing
  files.go                     Validation, revisions, backups, links, writes
  merge.go                     Merging JSON edits with outside changes
  undo.go                      Undo of the last save
  skills.go                    Skill folder checks and removal
  paths.go                     File identifiers and project boundaries
  projects.go                  Project folder suggestions
  server.go                    HTTP routes and access controls
  messages.go                  Conversation text normalization
  sessions.go                  Session routes, paging, and project filter
  sessions_jsonl.go            Codex, Claude, and Grok history and its cache
  sessions_sqlite.go           OpenCode history and Agy excerpts
web/
  src/main.tsx                 Primer theme setup
  src/App.tsx                  Header, tabs, lists, sessions, and editor panel
  src/components.tsx           File rows, change comparison, message text
  src/style.css                Layout on Primer design tokens
  src/api.ts                   Authenticated API client
  src/types.ts                 Interface data types
  tests/app.spec.ts            Browser integration tests
```

Go's standard HTTP library serves the API and compiled interface. Primer React supplies the interface components, design tokens, and Octicons. CodeMirror supplies the editor and change comparison. TOML, JSONC, SQLite, and `sjson` libraries handle the agents' storage formats.

## Development

### Build and check

```sh
make build
make test
```

`make build` installs the locked frontend dependencies, type-checks and bundles the interface, and produces `bin/agent-desk`. `make test` builds the app, runs Go tests with the race detector, and runs `go vet`.

The Go race detector requires a supported platform and a C compiler.

The UI must be built before compiling or testing the Go entry point because the executable embeds `web/dist`. Generated assets, binaries, dependency directories, and browser-test output are excluded from Git.

### Browser tests

Install Playwright's Chromium once, then run the integration tests:

```sh
cd web
npx playwright install chromium
cd ..
make test-e2e
```

Alternatively, point the tests at an existing Chromium-compatible executable:

```sh
CHROMIUM_PATH=/path/to/chromium make test-e2e
```

If `/usr/bin/google-chrome` exists, the test configuration uses it as a local fallback. Otherwise, Playwright uses its installed Chromium.

The browser suite starts the compiled app against a temporary fixture home. It checks agent navigation, instruction editing and review, undo, skill creation and removal, JSON merging, editing through linked folders, paged transcript viewing, keyboard shortcuts, invalid configuration, conflicting edits, and a narrow viewport. It does not edit your real agent configuration.

Backend tests cover configuration discovery, JSONL message parsing and session titles, both OpenCode database schemas, Agy excerpts, API access controls, read-only database access, backups and their pruning, JSON merging, undo, skill validation and removal, symbolic links, project suggestions, caching, paging, and path boundaries.

### Contributing changes

Keep agent-specific path and storage assumptions explicit. Add fixture-based coverage when changing a parser or write behavior. Use synthetic configuration and conversation data in tests; never commit personal session history or credentials.

For compatibility reports, include the agent version, operating system, file format, and a minimal sanitized example. Remove tokens, private conversation text, and account metadata before sharing logs or configuration.

## Limitations

- Inventory is based on known paths. It does not fully resolve runtime flags, environment overrides, managed remote settings, compatibility imports, custom skill directories, or plugin activation.
- Project discovery covers known files at the selected root. It does not recursively discover every nested instruction file or calculate inherited instruction precedence.
- Plugin installation, marketplace browsing, one-click extension toggles, and in-app agent execution are not implemented.
- Most transcript views show conversation text and omit tool events, system messages, and attachments. Model reasoning, where stored, appears in a collapsed **Thinking** box. Agy uses the excerpt format described above.
- JSONL transcript reads stop after **64 MB** and display a truncation notice. A single JSONL record larger than **64 MB** is skipped and counted in the notice.
- Conversations load 200 messages at a time, and the message filter searches the whole conversation. Messages longer than 20,000 characters are shortened until expanded.
- File editing is limited to **4 MB** per file.
- Undo covers the last save of each file and is lost when Agent Desk restarts; backups remain on disk.
- Session-list search covers titles, project paths, and session IDs. Message filtering applies to the selected conversation; there is no global full-text conversation search.
- Agent formats can change between releases. Support reflects the formats inspected during initial development, not a guarantee across every version.

## Adapter references

The initial adapters were developed against local configuration files and storage schemas, with these supporting references:

- [OpenAI MCP configuration](https://developers.openai.com/learn/docs-mcp).
- [OpenAI plugin structure](https://developers.openai.com/plugins/build/plugins).
- Grok's installed user guide: `~/.grok/docs/user-guide/`.
- Agy's installed customization guide: `~/.gemini/antigravity-cli/builtin/skills/agy-customizations/`.

Report bugs and compatibility gaps through [GitHub Issues](https://github.com/pradhankukiran/agent-desk/issues).
