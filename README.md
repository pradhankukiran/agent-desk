# Agent Desk

**A local workspace for your coding agents.**

Browse and edit the configuration behind Codex, Claude Code, OpenCode, Grok, and Agy from one browser interface. Inspect extensions, maintain instruction files, and explore conversation history while keeping each agent's native files as the source of truth.

Agent Desk runs as a Go executable with an embedded React interface. It binds to localhost and opens your browser. No cloud account, hosted backend, or agent API key is required.

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

Agent Desk starts at `http://localhost:7437` and attempts to open your browser. If automatic opening is unavailable, use the full URL printed in the terminal. Press **Ctrl+C** to stop the server.

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
| Agent overview | Installation detection through `PATH`, configuration inventory, and extension counts. |
| Project scope | User configuration alongside known project configuration and instruction files. |
| Extensions | MCP server declarations, skills, plugins, hooks, agent definitions, commands, and rules in supported locations. |
| Instructions | View, create, edit, and remove supported instruction files, including `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md`. |
| Configuration | Edit original JSON, JSONC, and TOML files with syntax validation and review before saving. |
| Skills | Create a skill, edit its `SKILL.md`, or remove an editable skill definition. |
| Sessions | Browse conversations, search titles and project paths, filter messages, and copy message text. |
| Change protection | Revision checks, automatic backups, permission preservation, and reviewable undo of the last save. |

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

Shared skills may appear under multiple agents. Bundled, installed-plugin, managed, and linked files are generally view-only. Supported user and project files can be edited.

### Agy conversation history

Agy stores full conversation steps as binary payloads. Agent Desk reads the local search index instead. These excerpts may omit content and group user and assistant text by index chunk. The interface labels them as excerpts rather than a complete, ordered transcript.

## Everyday use

1. Select an agent in the sidebar.
2. Keep **User configuration** selected, or choose a project to include its local files.
3. Open **Extensions**, **Instructions**, or **Settings** to inspect an item and its source path.
4. Make changes in the editor, select **Review changes**, then **Confirm save**.
5. Open **Sessions** to browse history and filter the messages in a selected conversation.

Use **Refresh** to rescan configuration and reload session metadata. An already-open editor retains its snapshot; close and reopen it to load external changes.

### Command-line options

| Option | Default | Purpose |
| --- | --- | --- |
| `--port` | `7437` | Choose the listening port. Set `0` to select an available port. |
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
- Writes outside the selected home and writes through symbolic links are rejected.
- Session databases are opened in read-only mode.
- The app does not intentionally select credential-only files such as `auth.json`. Configuration and session text can still contain credentials or other private information.
- The app serves local assets and does not send configuration or conversation data to an external service.

Do not expose the server through a public proxy or share its access URL. This is a local, single-user application.

### Saves and backups

Saving validates configuration syntax, checks the opened revision, preserves existing file permissions, and replaces the target through a temporary file and rename. Removing an editable skill or instruction file also creates a backup.

Backups are private JSON files stored under:

```text
~/.local/state/agent-desk/backups/
```

Each backup records the original `path`, `content`, and save time. **Undo last save** places the previous content into the review flow while the editor remains open. Older backups can be restored manually using their recorded path and content.

Backups may contain sensitive values. Revision checks reduce accidental overwrites, but they are not a locking protocol shared with running agent processes. Validation checks syntax, not every agent-specific setting or its operational effect.

## Architecture

```text
Browser: React + TypeScript + CodeMirror
                   │
              Local HTTP API
                   │
             Go application
                   │
    Agent configuration and session readers
                   │
      Native files / read-only SQLite
```

The app does not create a separate configuration database. Existing agent files remain authoritative; application-owned state consists of backups.

```text
main.go                        CLI, embedded assets, browser launch
internal/desk/
  agents.go                    Agent-specific discovery locations
  catalog.go                   File and extension inventory
  config.go                    JSON / JSONC / TOML parsing
  files.go                     Validation, revisions, backups, writes
  paths.go                     File identifiers and project boundaries
  server.go                    HTTP routes and access controls
  messages.go                  Conversation text normalization
  sessions_jsonl.go            Codex, Claude, and Grok history
  sessions_sqlite.go           OpenCode history and Agy excerpts
web/
  src/App.tsx                  Agent workspace and editor flows
  src/api.ts                   Authenticated API client
  src/types.ts                 Interface data types
  src/components.tsx           Shared file and empty-state views
  tests/app.spec.ts            Browser integration tests
```

Go's standard HTTP library serves the API and compiled interface. CodeMirror supplies the browser editor. TOML, JSONC, and SQLite libraries handle the agents' storage formats.

