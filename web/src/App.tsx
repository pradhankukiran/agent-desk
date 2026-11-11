import { useEffect, useRef, useState } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { javascript } from "@codemirror/lang-javascript";
import { StreamLanguage } from "@codemirror/language";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import type { Catalog, OpenFile, Session, Transcript } from "./types";
import { api, query } from "./api";
import { FileRow, Empty } from "./components";

const glyphs: Record<string, string> = {
  codex: "C",
  claude: "✳",
  opencode: "O",
  grok: "G",
  agy: "A",
};
const accents: Record<string, string> = {
  codex: "#347d73",
  claude: "#c67550",
  opencode: "#526c9a",
  grok: "#787087",
  agy: "#558769",
};
// languageFor picks syntax colouring from a file name.
function languageFor(name: string) {
  const ext = name.slice(name.lastIndexOf(".") + 1).toLowerCase();
  if (ext === "md") return [markdown()];
  if (ext === "toml") return [StreamLanguage.define(toml)];
  if (ext === "json" || ext === "jsonc") return [json()];
  if (["js", "mjs", "cjs", "ts", "mts", "cts"].includes(ext))
    return [javascript({ typescript: ext.endsWith("ts") })];
  return [];
}
// Shown in the sidebar until the catalog loads.
const defaultAgents = [
  { id: "codex", name: "Codex" },
  { id: "claude", name: "Claude Code" },
  { id: "opencode", name: "OpenCode" },
  { id: "grok", name: "Grok" },
  { id: "agy", name: "Agy" },
];
const tabs = ["Overview", "Extensions", "Instructions", "Settings", "Sessions"];
export default function App() {
  const [catalog, setCatalog] = useState<Catalog>();
  const [agentID, setAgentID] = useState("codex");
  const [project, setProject] = useState("");
  const [tab, setTab] = useState("Overview");
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState("all");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [reload, setReload] = useState(0);
  const [opened, setOpened] = useState<OpenFile>();
  const [draft, setDraft] = useState("");
  const [undoReview, setUndoReview] = useState(false);
  const [review, setReview] = useState(false);
  const [fileLoading, setFileLoading] = useState(false);
  const fileRequest = useRef(0);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [session, setSession] = useState<Session>();
  const [transcript, setTranscript] = useState<Transcript>();
  const [sessionBusy, setSessionBusy] = useState(false);
  const [sessionWarnings, setSessionWarnings] = useState<string[]>([]);
  const [transcriptSearch, setTranscriptSearch] = useState("");
  const [newSkill, setNewSkill] = useState(false);
  const [skillName, setSkillName] = useState("");
  const [skillDescription, setSkillDescription] = useState("");
  const [customProject, setCustomProject] = useState(false);
  const [projectDraft, setProjectDraft] = useState("");
  const agent = catalog?.agents.find((a) => a.id === agentID);
  const navAgents = catalog?.agents || defaultAgents;
  const dirty = !!opened && draft !== opened.content;
  useEffect(() => {
    let active = true;
    setError("");
    api<Catalog>("catalog?" + query({ project }))
      .then((c) => {
        if (active) setCatalog(c);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [project, reload]);
  useEffect(() => {
    if (!dirty) return;
    const f = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", f);
    return () => window.removeEventListener("beforeunload", f);
  }, [dirty]);
  useEffect(() => {
    if (tab !== "Sessions") return;
    let active = true;
    setSessionBusy(true);
    setSessions([]);
    setSession(undefined);
    setTranscript(undefined);
    api<{ sessions: Session[]; warnings: string[] }>(
      "sessions?" + query({ agent: agentID, project }),
    )
      .then((d) => {
        if (active) {
          setSessions(d.sessions);
          setSessionWarnings(d.warnings);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      })
      .finally(() => {
        if (active) setSessionBusy(false);
      });
    return () => {
      active = false;
    };
  }, [agentID, project, tab, reload]);
  useEffect(() => {
    if (!session) return;
    let active = true;
    setTranscript(undefined);
    api<Transcript>("session?" + query({ agent: agentID, id: session.id }))
      .then((d) => {
        if (active) setTranscript(d);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [session, agentID]);
  function mayLeave() {
    return !dirty || window.confirm("Discard your unsaved changes?");
  }
  function closeEditor() {
    if (!mayLeave()) return;
    fileRequest.current++;
    setOpened(undefined);
    setReview(false);
    setUndoReview(false);
    setFileLoading(false);
  }
  function navigate(id: string) {
    if (!mayLeave()) return;
    fileRequest.current++;
    setOpened(undefined);
    setReview(false);
    setAgentID(id);
    setSearch("");
    setError("");
  }
  async function openFile(id: string) {
    if (!mayLeave()) return;
    const req = ++fileRequest.current;
    setFileLoading(true);
    setOpened(undefined);
    setError("");
    try {
      await loadFile(id, req);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      if (req === fileRequest.current) setFileLoading(false);
    }
  }
  // loadFile shows the latest copy of a file, including whether undo is available.
  async function loadFile(id: string, req = ++fileRequest.current) {
    const d = await api<OpenFile>("file?" + query({ id, project }));
    if (req === fileRequest.current) {
      setOpened(d);
      setDraft(d.content);
      setReview(false);
      setUndoReview(false);
    }
  }
  async function save(content = draft) {
    if (!opened) return;
    setBusy(true);
    setError("");
    try {
      const d = await api<{
        revision: string;
        backup: string;
        merged: boolean;
      }>("file?" + query({ id: opened.file.id, project }), {
        method: "PUT",
        body: JSON.stringify({
          content,
          revision: opened.revision,
          base: opened.content,
        }),
      });
      await loadFile(opened.file.id);
      setNotice(
        d.merged
          ? "Saved. Another program changed this file while you were editing, so your changes were combined with its changes. A backup is in ~/.local/state/agent-desk/backups."
          : d.backup
            ? "Saved. A backup of the previous file is available in ~/.local/state/agent-desk/backups."
            : "File created.",
      );
      setReload((v) => v + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function undo() {
    if (!opened) return;
    setBusy(true);
    setError("");
    try {
      const d = await api<{ removed: boolean }>(
        "undo?" + query({ id: opened.file.id, project }),
        { method: "POST", body: JSON.stringify({ revision: opened.revision }) },
      );
      if (d.removed) {
        fileRequest.current++;
        setOpened(undefined);
        setUndoReview(false);
        setNotice("Undone. The file created by the last save was removed.");
      } else {
        await loadFile(opened.file.id);
        setNotice(
          "Undone. The file is back to how it was before the last save.",
        );
      }
      setReload((v) => v + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (!opened) return;
    const skill = opened.file.kind === "skill";
    if (opened.skillProblem) {
      setError("This skill cannot be removed here: " + opened.skillProblem);
      return;
    }
    const n = opened.skillFiles ?? 1;
    const question = skill
      ? `Remove this skill and its folder (${n} ${n === 1 ? "file" : "files"})? A backup copy will be saved.`
      : "Remove this file? A backup will be saved.";
    if (!window.confirm(question)) return;
    setBusy(true);
    setError("");
    try {
      await api("file?" + query({ id: opened.file.id, project }), {
        method: "DELETE",
        body: JSON.stringify({ revision: opened.revision }),
      });
      setOpened(undefined);
      setNotice(
        (skill ? "Skill removed." : "File removed.") +
          " A copy is saved in ~/.local/state/agent-desk/backups.",
      );
      setReload((v) => v + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function createSkill() {
    setBusy(true);
    setError("");
    try {
      const content = `---\nname: ${skillName}\ndescription: ${JSON.stringify(skillDescription)}\n---\n\n# ${skillName}\n\nDescribe the steps this skill should follow.\n`;
      const d = await api<{ id: string }>("skill", {
        method: "POST",
        body: JSON.stringify({
          agent: agentID,
          project,
          name: skillName,
          content,
        }),
      });
      setNewSkill(false);
      setReload((v) => v + 1);
      await openFile(d.id);
      setNotice("Skill created. Edit its instructions before using it.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function chooseProject(p: string) {
    if (!mayLeave()) return;
    fileRequest.current++;
    setOpened(undefined);
    setProject(p);
    setCustomProject(false);
  }
  const matches = (s: string) => s.toLowerCase().includes(search.toLowerCase());
  const ext =
    agent?.extensions.filter(
      (x) =>
        (kind === "all" || x.kind === kind) &&
        matches(x.name + " " + x.source + " " + x.scope),
    ) || [];
  const files =
    agent?.files.filter(
      (f) =>
        (tab === "Instructions"
          ? f.kind === "instruction"
          : f.kind === "config") && matches(f.path),
    ) || [];
  const visibleSessions = sessions.filter((s) =>
    matches(s.title + " " + s.project + " " + s.id),
  );
  const count = (k: string) =>
    agent?.extensions.filter((x) => x.kind === k).length || 0;
  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">▦</span>
          <div>
            agent desk<small>YOUR LOCAL WORKSPACE</small>
          </div>
        </div>
        <div className="nav-caption">
          AGENTS <span>{String(navAgents.length).padStart(2, "0")}</span>
        </div>
        <nav>
          {navAgents.map((a) => (
            <button
              key={a.id}
              className={"agent-nav " + (a.id === agentID ? "selected" : "")}
              onClick={() => navigate(a.id)}
            >
              <span className="agent-icon" style={{ color: accents[a.id] }}>
                {glyphs[a.id]}
              </span>
              <span>{a.name}</span>
              <span className="nav-dot" />
            </button>
          ))}
        </nav>
        <div className="sidebar-note">
          <span className="live-dot" /> Running locally
          <p>
            Your agents. Your files.
            <br />
            One place to see it all.
          </p>
        </div>
        <div className="sidebar-footer">
          LOCAL WORKSPACE <span>↗</span>
        </div>
      </aside>
      <main>
        <header className="topbar">
          <span>
            Workspace <span className="slash">/</span>{" "}
            <strong>{agent?.name || "Loading"}</strong>
          </span>
          <span className="local-pill">
            <span className="live-dot" /> Local connection
          </span>
        </header>
        <div className="workspace">
          <div className="page-heading">
            <div>
              <div className="eyebrow">AGENT WORKSPACE</div>
              <h1>
                {agent?.name || "Agent Desk"}
                <span className="status-tag">
                  {agent?.installed ? "Installed" : "Not found"}
                </span>
              </h1>
              <p>
                Configuration, extensions, and conversations. All in one place.
              </p>
            </div>
            <button
              className="button secondary"
              onClick={() => {
                setReload((v) => v + 1);
                setNotice("Workspace refreshed.");
              }}
            >
              ↻ Refresh
            </button>
          </div>
          <div className="scopebar">
            <span className="scope-label">◎ SCOPE</span>
            <select
              aria-label="Project scope"
              value={project}
              onChange={(e) =>
                e.target.value === "__custom"
                  ? setCustomProject(true)
                  : chooseProject(e.target.value)
              }
            >
              <option value="">User configuration · all projects</option>
              {Array.from(
                new Set([
                  ...(catalog?.projects || []),
                  ...(project ? [project] : []),
                ]),
              ).map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
              <option value="__custom">Choose another directory…</option>
            </select>
            <span className="scope-hint">
              {project
                ? "User + project files"
                : "Select a project to include its settings"}
            </span>
          </div>
          <div className="tabs" role="tablist">
            {tabs.map((t) => (
              <button
                role="tab"
                aria-selected={t === tab}
                key={t}
                className={t === tab ? "active" : ""}
                onClick={() => {
                  setTab(t);
                  setSearch("");
                  setKind("all");
                }}
              >
                {t}
              </button>
            ))}
          </div>
          {error && (
            <div role="alert" className="alert error">
              {error}
              <button onClick={() => setError("")}>×</button>
            </div>
          )}
          {notice && (
            <div role="status" className="alert success">
              {notice}
              <button onClick={() => setNotice("")}>×</button>
            </div>
          )}
          {agent?.warnings.map((w, i) => (
            <div className="alert warning" key={i}>
              {w}
            </div>
          ))}
          {tab === "Overview" && agent && (
            <>
              <div className="stats">
                {[
                  ["mcp", "MCP servers", "Connected tool configuration"],
                  ["skill", "Skills", "Instructions available on disk"],
                  ["plugin", "Plugins", "Configured or installed packages"],
                ].map(([k, label, hint]) => (
                  <button
                    className="stat"
                    key={k}
                    onClick={() => {
                      setTab("Extensions");
                      setKind(k);
                    }}
                  >
                    <div>
                      {label}
                      <span>↗</span>
                    </div>
                    <strong>{count(k).toString().padStart(2, "0")}</strong>
                    <small>{hint}</small>
                  </button>
                ))}
              </div>
              <div className="overview-grid">
                <section className="panel">
                  <div className="panel-heading">
                    <h2>Configuration files</h2>
                    <button
                      className="text-button"
                      onClick={() => setTab("Settings")}
                    >
                      View all →
                    </button>
                  </div>
                  {agent.files
                    .filter((f) => f.kind === "config")
                    .map((f) => (
                      <FileRow
                        key={f.id}
                        file={f}
                        onOpen={() => openFile(f.id)}
                      />
                    ))}
                </section>
                <section className="panel">
                  <div className="panel-heading">
                    <h2>Workspace details</h2>
                    <span className="subtle">ON THIS MACHINE</span>
                  </div>
                  <dl>
                    <dt>Executable</dt>
                    <dd>{agent.executable || "Not found on PATH"}</dd>
                    <dt>Configuration source</dt>
                    <dd>Native agent files</dd>
                    <dt>Editing</dt>
                    <dd>Validated saves with automatic backups</dd>
                    <dt>Session access</dt>
                    <dd>
                      {agent.id === "agy"
                        ? "Local search index excerpts"
                        : "Local conversation history"}
                    </dd>
                  </dl>
                  <div className="info-box">
                    “Present” means found on disk. It does not confirm that an
                    agent has loaded or enabled an extension.
                  </div>
                </section>
              </div>
              <section className="panel instructions-preview">
                <div>
                  <div className="eyebrow">MAKE IT YOURS</div>
                  <h2>Instructions that shape your agent</h2>
                  <p>
                    Read and edit your agent’s instruction files, with their
                    original location always visible.
                  </p>
                </div>
                <button
                  className="button secondary"
                  onClick={() => setTab("Instructions")}
                >
                  Open instructions →
                </button>
              </section>
            </>
          )}
          {tab !== "Overview" && (
            <div className="toolbar">
              <div className="search">
                <span>⌕</span>
                <input
                  aria-label="Search"
                  placeholder={
                    tab === "Sessions"
                      ? "Search titles, projects, or session IDs…"
                      : "Search names or file paths…"
                  }
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              {tab === "Extensions" && (
                <>
                  <select
                    aria-label="Extension type"
                    value={kind}
                    onChange={(e) => setKind(e.target.value)}
                  >
                    {[
                      "all",
                      "mcp",
                      "skill",
                      "plugin",
                      "hook",
                      "agent",
                      "command",
                      "rule",
                    ].map((k) => (
                      <option key={k} value={k}>
                        {k === "all"
                          ? "All types"
                          : k === "mcp"
                            ? "MCP servers"
                            : k.charAt(0).toUpperCase() + k.slice(1) + "s"}
                      </option>
                    ))}
                  </select>
                  <button
                    className="button primary"
                    onClick={() => {
                      setSkillName("");
                      setSkillDescription("");
                      setNewSkill(true);
                    }}
                  >
                    + New skill
                  </button>
                </>
              )}
            </div>
          )}
          {tab === "Extensions" && (
            <>
              <div className="section-caption">
                <span>{ext.length} ITEMS</span>
                <span>Open a source file to edit configuration</span>
              </div>
              <div className="extension-grid">
                {ext.map((x, i) => (
                  <button
                    className="extension-card"
                    key={x.fileId + x.name + i}
                    onClick={() => openFile(x.fileId)}
                  >
                    <div className="extension-top">
                      <span className={"kind-icon " + x.kind}>
                        {x.kind === "mcp"
                          ? "⌘"
                          : x.kind === "skill"
                            ? "✧"
                            : "▧"}
                      </span>
                      <span className={"badge " + x.status}>{x.status}</span>
                    </div>
                    <h3>{x.name}</h3>
                    <div className="extension-meta">
                      {x.kind === "mcp" ? "MCP server" : x.kind}
                      <span>·</span>
                      {x.scope}
                    </div>
                    <p title={x.source}>
                      {x.source.replace(catalog?.home || "", "~")}
                    </p>
                    <div className="card-footer">
                      View source <span>↗</span>
                    </div>
                  </button>
                ))}
              </div>
              {ext.length === 0 && (
                <Empty
                  title="No matching extensions"
                  text="Nothing was found in the supported configuration locations. You can add entries through Settings."
                />
              )}
              <div className="footnote">
                Inventory covers known local paths. Runtime overrides, remote
                catalogs, and extension activation may differ. Plugin
                installation is not available in this version.
              </div>
            </>
          )}
          {(tab === "Instructions" || tab === "Settings") && (
            <section className="panel">
              <div className="panel-heading">
                <h2>
                  {tab === "Instructions"
                    ? "Instruction files"
                    : "Configuration files"}
                </h2>
                <span className="subtle">{files.length} FILES</span>
              </div>
              {files.map((f) => (
                <FileRow key={f.id} file={f} onOpen={() => openFile(f.id)} />
              ))}
              {files.length === 0 && (
                <Empty
                  title="No matching files"
                  text="Try a different search or project."
                />
              )}
              <div className="panel-note">
                {tab === "Settings"
                  ? "Edit the original configuration to add or change MCP servers, plugins, hooks, and agent preferences. JSON and TOML are validated before saving."
                  : "Files marked “Not created” can be created here. Files are shown by location; this list does not imply instruction precedence."}
              </div>
            </section>
          )}
          {tab === "Sessions" && (
            <>
              {sessionWarnings.map((w, i) => (
                <div className="alert warning" key={i}>
                  {w}
                </div>
              ))}
              <div className="sessions-layout">
                <section className="panel session-list">
                  <div className="panel-heading">
                    <h2>Conversations</h2>
                    <span className="subtle">{visibleSessions.length}</span>
                  </div>
                  {sessionBusy ? (
                    <Empty
                      title="Reading local history…"
                      text="Loading session metadata."
                    />
                  ) : visibleSessions.length === 0 ? (
                    <Empty
                      title="No matching sessions"
                      text="Try another project or clear your search."
                    />
                  ) : (
                    visibleSessions.map((s) => (
                      <button
                        className={
                          "session-row " +
                          (s.id === session?.id ? "selected" : "")
                        }
                        key={s.id}
                        onClick={() => {
                          setSession(s);
                          setTranscriptSearch("");
                        }}
                      >
                        <strong>{s.title}</strong>
                        <span>{s.project || "Project not recorded"}</span>
                        <small>
                          {new Date(s.updated).toLocaleString()}{" "}
                          {s.format === "indexed excerpts"
                            ? "· Indexed excerpts"
                            : ""}
                        </small>
                      </button>
                    ))
                  )}
                </section>
                <section className="panel transcript">
                  {!session ? (
                    <Empty
                      title="Pick a conversation"
                      text="Read the history without changing the agent’s session files."
                    />
                  ) : (
                    <>
                      <div className="transcript-heading">
                        <h2>{session.title}</h2>
                        <input
                          aria-label="Find in conversation"
                          placeholder="Filter messages…"
                          value={transcriptSearch}
                          onChange={(e) => setTranscriptSearch(e.target.value)}
                        />
                      </div>
                      {!transcript ? (
                        <Empty
                          title="Loading conversation…"
                          text="Reading stored messages."
                        />
                      ) : (
                        <>
                          <div className="panel-note">
                            {transcript.note || "Stored conversation text."}
                          </div>
                          {transcript.messages.length === 0 && (
                            <Empty
                              title="No readable messages"
                              text="This session has no text available in the supported storage format."
                            />
                          )}
                          {transcript.messages
                            .filter((m) =>
                              m.text
                                .toLowerCase()
                                .includes(transcriptSearch.toLowerCase()),
                            )
                            .map((m, i) => {
                              const copy = (
                                <button
                                  onClick={(e) => {
                                    e.preventDefault();
                                    navigator.clipboard
                                      .writeText(m.text)
                                      .then(() => setNotice("Message copied."))
                                      .catch(() =>
                                        setError(
                                          "Clipboard access is unavailable.",
                                        ),
                                      );
                                  }}
                                >
                                  Copy
                                </button>
                              );
                              return m.role === "thinking" ? (
                                <details
                                  className="message thinking"
                                  key={i}
                                  open={transcriptSearch !== ""}
                                >
                                  <summary className="message-role">
                                    <span>Thinking</span>
                                    {copy}
                                  </summary>
                                  <pre>{m.text}</pre>
                                </details>
                              ) : (
                                <article
                                  className={"message " + m.role}
                                  key={i}
                                >
                                  <div className="message-role">
                                    {m.role}
                                    {copy}
                                  </div>
                                  <pre>{m.text}</pre>
                                </article>
                              );
                            })}
                        </>
                      )}
                    </>
                  )}
                </section>
              </div>
            </>
          )}
          <footer className="page-footer">
            AGENT DESK <span>Stored locally. Connected to your workspace.</span>
          </footer>
        </div>
      </main>
      {(opened || fileLoading) && (
        <div
          className="overlay"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget && !busy) closeEditor();
          }}
        >
          <section
            className="editor-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="File editor"
          >
            <div className="dialog-heading">
              <div>
                <span className="eyebrow">{opened?.file.kind || "FILE"}</span>
                <h2>
                  {opened?.file.name || "Loading…"}
                  {dirty && <span className="unsaved">Unsaved</span>}
                </h2>
              </div>
              <button
                className="close"
                disabled={busy}
                aria-label="Close editor"
                onClick={closeEditor}
              >
                ×
              </button>
            </div>
            {opened && (
              <>
                <div className="file-path">
                  {opened.file.path}
                  <span>
                    {opened.file.scope} ·{" "}
                    {opened.file.editable ? "editable" : "read-only"}
                  </span>
                </div>
                {error && (
                  <div role="alert" className="alert error">
                    {error}
                  </div>
                )}
                {opened.file.target && (
                  <div className="panel-note">
                    This path is a link. Changes are saved to{" "}
                    <code>{opened.file.target}</code>, and the link stays in
                    place.
                  </div>
                )}
                {!opened.file.editable && (
                  <div className="panel-note">
                    This is a managed, bundled, or plugin file, or it links
                    outside your home folder. It is available for viewing.
                  </div>
                )}
                {opened.file.name === ".claude.json" && (
                  <div className="panel-note">
                    Claude’s state file includes account metadata alongside MCP
                    settings. Edit only the fields you intend to change.
                  </div>
                )}
                {undoReview && opened.undo ? (
                  <div className="review-grid">
                    <div>
                      <h3>Current file</h3>
                      <pre>{opened.content}</pre>
                    </div>
                    <div>
                      <h3>After undo</h3>
                      <pre>
                        {opened.undo.removes
                          ? "(The file will be removed, because the last save created it.)"
                          : opened.undo.content || "(Empty file)"}
                      </pre>
                    </div>
                  </div>
                ) : review ? (
                  <div className="review-grid">
                    <div>
                      <h3>Current file</h3>
                      <pre>{opened.content || "(New file)"}</pre>
                    </div>
                    <div>
                      <h3>Proposed file</h3>
                      <pre>{draft}</pre>
                    </div>
                  </div>
                ) : (
                  <div className="code-editor">
                    <CodeMirror
                      value={draft}
                      height="100%"
                      extensions={languageFor(opened.file.name)}
                      editable={opened.file.editable && !busy}
                      onChange={setDraft}
                      basicSetup={{ lineNumbers: true, foldGutter: true }}
                    />
                  </div>
                )}
                <div className="editor-actions">
                  <div>
                    {opened.file.editable &&
                      opened.file.exists &&
                      ["skill", "instruction"].includes(opened.file.kind) && (
                        <button
                          className="button danger"
                          disabled={busy}
                          onClick={remove}
                        >
                          {opened.file.kind === "skill"
                            ? "Remove skill"
                            : "Remove file"}
                        </button>
                      )}
                    {opened.undo && !review && !undoReview && (
                      <button
                        className="button secondary"
                        disabled={busy || dirty}
                        title={dirty ? "Save or discard your edits first" : ""}
                        onClick={() => setUndoReview(true)}
                      >
                        Undo last save
                      </button>
                    )}
                  </div>
                  <div>
                    {undoReview && (
                      <>
                        <button
                          className="button secondary"
                          disabled={busy}
                          onClick={() => setUndoReview(false)}
                        >
                          Back to editing
                        </button>
                        <button
                          className="button primary"
                          disabled={busy}
                          onClick={undo}
                        >
                          {busy ? "Undoing…" : "Confirm undo"}
                        </button>
                      </>
                    )}
                    {review && (
                      <button
                        className="button secondary"
                        disabled={busy}
                        onClick={() => setReview(false)}
                      >
                        Back to editing
                      </button>
                    )}
                    {opened.file.editable && !undoReview && (
                      <button
                        className="button primary"
                        disabled={!dirty || busy}
                        onClick={() => (review ? save() : setReview(true))}
                      >
                        {busy
                          ? "Saving…"
                          : review
                            ? "Confirm save"
                            : "Review changes"}
                      </button>
                    )}
                  </div>
                </div>
              </>
            )}
          </section>
        </div>
      )}
      {newSkill && (
        <div className="overlay">
          <form
            className="small-dialog"
            onSubmit={(e) => {
              e.preventDefault();
              createSkill();
            }}
          >
            <div className="dialog-heading">
              <h2>Create a skill</h2>
              <button
                type="button"
                className="close"
                onClick={() => setNewSkill(false)}
              >
                ×
              </button>
            </div>
            <p>
              For {agent?.name} ·{" "}
              {project ? "selected project" : "user configuration"}
            </p>
            {error && <div className="alert error">{error}</div>}
            <label>
              Name
              <input
                required
                pattern="[a-z0-9][a-z0-9-]{0,63}"
                placeholder="review-changes"
                value={skillName}
                onChange={(e) => setSkillName(e.target.value)}
              />
            </label>
            <label>
              Description
              <textarea
                required
                placeholder="When should this skill be used?"
                value={skillDescription}
                onChange={(e) => setSkillDescription(e.target.value)}
              />
            </label>
            <button className="button primary" disabled={busy}>
              Create and edit
            </button>
          </form>
        </div>
      )}
      {customProject && (
        <div className="overlay">
          <form
            className="small-dialog"
            onSubmit={async (e) => {
              e.preventDefault();
              try {
                await api("catalog?" + query({ project: projectDraft }));
                chooseProject(projectDraft);
              } catch (e) {
                setError((e as Error).message);
              }
            }}
          >
            <div className="dialog-heading">
              <h2>Choose a project</h2>
              <button
                type="button"
                className="close"
                onClick={() => setCustomProject(false)}
              >
                ×
              </button>
            </div>
            <label>
              Directory inside your home
              <input
                required
                placeholder="/home/you/projects/my-project"
                value={projectDraft}
                onChange={(e) => setProjectDraft(e.target.value)}
              />
            </label>
            {error && <div className="alert error">{error}</div>}
            <button className="button primary">Open project</button>
          </form>
        </div>
      )}
    </div>
  );
}
