import { useEffect, useRef, useState } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { javascript } from "@codemirror/lang-javascript";
import { StreamLanguage } from "@codemirror/language";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import type { Catalog, OpenFile, Session, Transcript } from "./types";
import { api, query } from "./api";
import { FileRow, Empty, Changes, MessageText } from "./components";

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
const tabs = ["Files", "Extensions", "Sessions"];
const kindNames: Record<string, string> = {
  mcp: "MCP server",
  skill: "Skill",
  plugin: "Plugin",
  hook: "Hook",
  agent: "Agent",
  command: "Command",
  rule: "Rule",
};
export default function App() {
  const [catalog, setCatalog] = useState<Catalog>();
  const [agentID, setAgentID] = useState("codex");
  const [project, setProject] = useState("");
  const [tab, setTab] = useState("Files");
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
  const searchBox = useRef<HTMLInputElement>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [session, setSession] = useState<Session>();
  const [transcript, setTranscript] = useState<Transcript>();
  const [sessionBusy, setSessionBusy] = useState(false);
  const [sessionWarnings, setSessionWarnings] = useState<string[]>([]);
  const [transcriptSearch, setTranscriptSearch] = useState("");
  const [transcriptQuery, setTranscriptQuery] = useState("");
  const [moreBusy, setMoreBusy] = useState(false);
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
  // Filter on the server shortly after typing stops.
  useEffect(() => {
    const t = setTimeout(() => setTranscriptQuery(transcriptSearch), 250);
    return () => clearTimeout(t);
  }, [transcriptSearch]);
  const transcriptPath = (offset: number) =>
    "session?" +
    query({
      agent: agentID,
      id: session?.id || "",
      q: transcriptQuery,
      offset: String(offset),
    });
  useEffect(() => {
    if (!session) return;
    let active = true;
    setTranscript(undefined);
    api<Transcript>(transcriptPath(0))
      .then((d) => {
        if (active) setTranscript(d);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [session, agentID, transcriptQuery]);
  async function loadMoreMessages() {
    if (!transcript) return;
    const shown = transcript;
    setMoreBusy(true);
    try {
      const d = await api<Transcript>(transcriptPath(shown.messages.length));
      setTranscript((t) =>
        t === shown ? { ...d, messages: [...t.messages, ...d.messages] } : t,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setMoreBusy(false);
    }
  }
  // Keyboard: "/" jumps to search; Escape closes the open dialog or editor.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const typing =
        target.closest("input, textarea, select, .cm-editor") !== null;
      if (e.key === "/" && !typing && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        searchBox.current?.focus();
      } else if (e.key === "Escape") {
        if (newSkill) setNewSkill(false);
        else if (customProject) setCustomProject(false);
        else if ((opened || fileLoading) && !busy) closeEditor();
        else if (target === searchBox.current) searchBox.current?.blur();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });
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
  const fileGroups = [
    { title: "Settings", kind: "config" },
    { title: "Instructions", kind: "instruction" },
  ].map((g) => ({
    ...g,
    files:
      agent?.files.filter((f) => f.kind === g.kind && matches(f.path)) || [],
  }));
  const fileCount =
    agent?.files.filter((f) => f.kind === "config" || f.kind === "instruction")
      .length || 0;
  const tilde = (p: string) =>
    catalog?.home && p.startsWith(catalog.home)
      ? "~" + p.slice(catalog.home.length)
      : p;
  const visibleSessions = sessions.filter((s) =>
    matches(s.title + " " + s.project + " " + s.id),
  );
  return (
    <div className={"app" + (opened || fileLoading ? " with-panel" : "")}>
      <aside className="sidebar">
        <div className="brand">agent desk</div>
        <div className="nav-caption">
          Agents <span>{navAgents.length}</span>
        </div>
        <nav>
          {navAgents.map((a) => {
            const full = catalog?.agents.find((x) => x.id === a.id);
            return (
              <button
                key={a.id}
                className={"agent-nav" + (a.id === agentID ? " selected" : "")}
                aria-current={a.id === agentID ? "page" : undefined}
                onClick={() => navigate(a.id)}
              >
                <span
                  className={"install-dot" + (full?.installed ? " on" : "")}
                  title={full?.installed ? "Installed" : "Not found on PATH"}
                />
                <span className="agent-name">{a.name}</span>
                {full && (
                  <span className="agent-count" title="Extensions found">
                    {full.extensions.length}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      </aside>
      <main>
        <header className="page-header">
          <div>
            <h1>{agent?.name || "Agent Desk"}</h1>
            <p className="page-meta">
              {!agent
                ? "Loading…"
                : agent.installed
                  ? agent.executable
                  : "Not found on PATH"}
            </p>
          </div>
          <div className="header-actions">
            <label className="scope-label" htmlFor="project-scope">
              Project
            </label>
            <select
              id="project-scope"
              aria-label="Project scope"
              value={project}
              onChange={(e) =>
                e.target.value === "__custom"
                  ? setCustomProject(true)
                  : chooseProject(e.target.value)
              }
            >
              <option value="">None (user settings only)</option>
              {Array.from(
                new Set([
                  ...(catalog?.projects || []),
                  ...(project ? [project] : []),
                ]),
              ).map((p) => (
                <option key={p} value={p}>
                  {tilde(p)}
                </option>
              ))}
              <option value="__custom">Other folder…</option>
            </select>
            <button
              className="button secondary"
              onClick={() => {
                setReload((v) => v + 1);
                setNotice("Reloaded from disk.");
              }}
            >
              Reload
            </button>
          </div>
        </header>
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
              {t !== "Sessions" && agent && (
                <span className="tab-count">
                  {t === "Files" ? fileCount : agent.extensions.length}
                </span>
              )}
            </button>
          ))}
        </div>
        <div className="content">
          {error && !opened && (
            <div role="alert" className="alert error">
              {error}
              <button aria-label="Dismiss" onClick={() => setError("")}>
                ×
              </button>
            </div>
          )}
          {notice && (
            <div role="status" className="alert success">
              {notice}
              <button aria-label="Dismiss" onClick={() => setNotice("")}>
                ×
              </button>
            </div>
          )}
          {agent?.warnings.map((w, i) => (
            <div className="alert warning" key={i}>
              {w}
            </div>
          ))}
          <div className="toolbar">
            <input
              ref={searchBox}
              className="search"
              type="search"
              aria-label="Search"
              placeholder={
                tab === "Sessions"
                  ? "Search titles and folders  ( / )"
                  : tab === "Files"
                    ? "Search file paths  ( / )"
                    : "Search names and paths  ( / )"
              }
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {tab === "Extensions" && (
              <>
                <select
                  aria-label="Extension type"
                  value={kind}
                  onChange={(e) => setKind(e.target.value)}
                >
                  <option value="all">All types</option>
                  {Object.entries(kindNames).map(([k, name]) => (
                    <option key={k} value={k}>
                      {name}s
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
                  New skill
                </button>
              </>
            )}
          </div>
          {tab === "Files" &&
            fileGroups.map((g) => (
              <section className="list-section" key={g.kind}>
                <h2>
                  {g.title} <span>{g.files.length}</span>
                </h2>
                {g.files.length === 0 ? (
                  <Empty title="No matching files" text="" />
                ) : (
                  <div className="list">
                    {g.files.map((f) => (
                      <FileRow
                        key={f.id}
                        file={f}
                        path={tilde(f.path)}
                        selected={opened?.file.id === f.id}
                        onOpen={() => openFile(f.id)}
                      />
                    ))}
                  </div>
                )}
              </section>
            ))}
          {tab === "Files" && (
            <p className="footnote">
              Missing files can be created by opening them. Files are listed by
              location, not by the order an agent loads them.
            </p>
          )}
          {tab === "Extensions" && (
            <>
              {ext.length === 0 ? (
                <Empty
                  title="No matching extensions"
                  text="Nothing was found in the supported locations."
                />
              ) : (
                <div className="list">
                  {ext.map((x, i) => (
                    <button
                      className={
                        "row extension-row" +
                        (opened?.file.id === x.fileId ? " selected" : "")
                      }
                      key={x.fileId + x.name + i}
                      onClick={() => openFile(x.fileId)}
                    >
                      <span className="row-main">
                        <strong>{x.name}</strong>
                        <small title={x.source}>{tilde(x.source)}</small>
                      </span>
                      <span className="row-meta">{kindNames[x.kind]}</span>
                      <span className="row-meta">{x.scope}</span>
                      <span className={"row-meta status-" + x.status}>
                        {x.status}
                      </span>
                    </button>
                  ))}
                </div>
              )}
              <p className="footnote">
                “Present” means found on disk; “configured” means listed in a
                settings file. Neither confirms the agent has loaded it.
              </p>
            </>
          )}
          {tab === "Sessions" && (
            <>
              {sessionWarnings.map((w, i) => (
                <div className="alert warning" key={i}>
                  {w}
                </div>
              ))}
              <div className="sessions-layout">
                <section className="session-list">
                  {sessionBusy ? (
                    <Empty title="Loading…" text="" />
                  ) : visibleSessions.length === 0 ? (
                    <Empty
                      title="No matching sessions"
                      text="Try another project or clear your search."
                    />
                  ) : (
                    visibleSessions.map((s) => (
                      <button
                        className={
                          "session-row" +
                          (s.id === session?.id ? " selected" : "")
                        }
                        key={s.id}
                        onClick={() => {
                          setSession(s);
                          setTranscriptSearch("");
                          setTranscriptQuery("");
                        }}
                      >
                        <strong>{s.title}</strong>
                        <small>
                          {s.project ? tilde(s.project) : "Folder not recorded"}
                        </small>
                        <small>
                          {new Date(s.updated).toLocaleString()}
                          {s.format === "indexed excerpts" ? " · Excerpts" : ""}
                        </small>
                      </button>
                    ))
                  )}
                </section>
                <section className="transcript">
                  {!session ? (
                    <Empty
                      title="Pick a conversation"
                      text="Conversations are read-only."
                    />
                  ) : (
                    <>
                      <div className="transcript-heading">
                        <h2>{session.title}</h2>
                        <input
                          type="search"
                          aria-label="Find in conversation"
                          placeholder="Filter messages"
                          value={transcriptSearch}
                          onChange={(e) => setTranscriptSearch(e.target.value)}
                        />
                      </div>
                      {!transcript ? (
                        <Empty title="Loading…" text="" />
                      ) : (
                        <>
                          <p className="footnote">
                            {transcript.note || "Stored conversation text."}
                          </p>
                          {transcript.messages.length === 0 &&
                            (transcriptQuery ? (
                              <Empty
                                title="No matching messages"
                                text="Try different words."
                              />
                            ) : (
                              <Empty
                                title="No readable messages"
                                text="This session has no text in a supported format."
                              />
                            ))}
                          {transcript.messages.map((m, i) => {
                            const copy = (
                              <button
                                className="text-button"
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
                                open={transcriptQuery !== ""}
                              >
                                <summary className="message-role">
                                  <span>Thinking</span>
                                  {copy}
                                </summary>
                                <MessageText text={m.text} />
                              </details>
                            ) : (
                              <article className={"message " + m.role} key={i}>
                                <div className="message-role">
                                  <span>{m.role}</span>
                                  {copy}
                                </div>
                                <MessageText text={m.text} />
                              </article>
                            );
                          })}
                          {transcript.total > 0 && (
                            <div className="transcript-more">
                              Showing {transcript.messages.length} of{" "}
                              {transcript.total}{" "}
                              {transcriptQuery
                                ? "matching messages"
                                : "messages"}
                              {transcript.messages.length <
                                transcript.total && (
                                <button
                                  className="button secondary"
                                  disabled={moreBusy}
                                  onClick={loadMoreMessages}
                                >
                                  {moreBusy ? "Loading…" : "Show more"}
                                </button>
                              )}
                            </div>
                          )}
                        </>
                      )}
                    </>
                  )}
                </section>
              </div>
            </>
          )}
        </div>
      </main>
      {(opened || fileLoading) && (
        <section
          className="editor-panel"
          role="dialog"
          aria-label="File editor"
        >
          <div className="panel-header">
            <div>
              <h2>
                {opened?.file.name || "Loading…"}
                {dirty && <span className="unsaved">Unsaved</span>}
              </h2>
              {opened && (
                <p className="file-path" title={opened.file.path}>
                  {tilde(opened.file.path)}
                  <span>
                    {" · "}
                    {opened.file.scope} ·{" "}
                    {opened.file.editable ? "editable" : "read-only"}
                  </span>
                </p>
              )}
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
              {error && (
                <div role="alert" className="alert error">
                  {error}
                </div>
              )}
              {opened.file.target && (
                <p className="panel-note">
                  This path is a link. Changes are saved to{" "}
                  <code>{tilde(opened.file.target)}</code>, and the link stays
                  in place.
                </p>
              )}
              {!opened.file.editable && (
                <p className="panel-note">
                  This is a managed, bundled, or plugin file, or it links
                  outside your home folder. It is available for viewing.
                </p>
              )}
              {opened.file.name === ".claude.json" && (
                <p className="panel-note">
                  Claude’s state file also holds account details. Edit only the
                  fields you intend to change.
                </p>
              )}
              {undoReview && opened.undo ? (
                opened.undo.removes ? (
                  <div className="review-summary removal">
                    <h3>After undo</h3>
                    <span>
                      The file will be removed, because the last save created
                      it.
                    </span>
                  </div>
                ) : (
                  <Changes
                    title="After undo"
                    before={opened.content}
                    after={opened.undo.content}
                    language={languageFor(opened.file.name)}
                  />
                )
              ) : review ? (
                <Changes
                  title={opened.file.exists ? "Changes to save" : "New file"}
                  before={opened.content}
                  after={draft}
                  language={languageFor(opened.file.name)}
                />
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
      )}
      {newSkill && (
        <div className="overlay">
          <form
            className="small-dialog"
            role="dialog"
            aria-label="Create a skill"
            onSubmit={(e) => {
              e.preventDefault();
              createSkill();
            }}
          >
            <div className="panel-header">
              <h2>New skill</h2>
              <button
                type="button"
                className="close"
                aria-label="Close"
                onClick={() => setNewSkill(false)}
              >
                ×
              </button>
            </div>
            <p className="panel-note">
              For {agent?.name},{" "}
              {project ? "in " + tilde(project) : "in your user settings"}.
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
            <div className="editor-actions">
              <span />
              <button className="button primary" disabled={busy}>
                Create and edit
              </button>
            </div>
          </form>
        </div>
      )}
      {customProject && (
        <div className="overlay">
          <form
            className="small-dialog"
            role="dialog"
            aria-label="Choose a project"
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
            <div className="panel-header">
              <h2>Choose a project</h2>
              <button
                type="button"
                className="close"
                aria-label="Close"
                onClick={() => setCustomProject(false)}
              >
                ×
              </button>
            </div>
            <label>
              Folder inside your home
              <input
                required
                placeholder="~/projects/my-project"
                value={projectDraft}
                onChange={(e) => setProjectDraft(e.target.value)}
              />
            </label>
            {error && <div className="alert error">{error}</div>}
            <div className="editor-actions">
              <span />
              <button className="button primary">Open project</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
