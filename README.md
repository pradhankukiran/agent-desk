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

