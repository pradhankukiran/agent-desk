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

