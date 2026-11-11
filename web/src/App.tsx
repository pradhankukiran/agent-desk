import { useEffect, useRef, useState } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { javascript } from "@codemirror/lang-javascript";
import { StreamLanguage } from "@codemirror/language";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import type { Catalog, OpenFile, Session, Transcript } from "./types";
import { api, query } from "./api";
import { FileRow, Changes, MessageText, useDarkMode } from "./components";
import {
  Banner,
  Button,
  CounterLabel,
  FormControl,
  Heading,
  IconButton,
  Label,
  RelativeTime,
  Select,
  Spinner,
  TextInput,
  Textarea,
  UnderlineNav,
} from "@primer/react";
import { Blankslate, Dialog, KeybindingHint } from "@primer/react/experimental";
import {
  ChecklistIcon,
  CommentDiscussionIcon,
  CopyIcon,
  CpuIcon,
  FileIcon,
  HistoryIcon,
  LightBulbIcon,
  LinkIcon,
  LockIcon,
  PersonIcon,
  PlugIcon,
  PlusIcon,
  SearchIcon,
  ServerIcon,
  SyncIcon,
  TerminalIcon,
  TrashIcon,
  XIcon,
  ZapIcon,
  type Icon,
} from "@primer/octicons-react";

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
const tabIcons: Record<string, Icon> = {
  Files: FileIcon,
  Extensions: PlugIcon,
  Sessions: CommentDiscussionIcon,
};
const kindIcons: Record<string, Icon> = {
  mcp: ServerIcon,
  skill: LightBulbIcon,
  plugin: PlugIcon,
  hook: ZapIcon,
  agent: PersonIcon,
  command: TerminalIcon,
  rule: ChecklistIcon,
};
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
  const dark = useDarkMode();
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
      <header className="app-header">
        <div className="brand">
          <CpuIcon size={20} />
          <span>Agent Desk</span>
        </div>
        <nav className="agent-tabs" aria-label="Agents">
          {navAgents.map((a) => {
            const full = catalog?.agents.find((x) => x.id === a.id);
            return (
              <button
                key={a.id}
                className="agent-tab"
                aria-current={a.id === agentID ? "page" : undefined}
                onClick={() => navigate(a.id)}
              >
                <span
                  className={"install-dot" + (full?.installed ? " on" : "")}
                  title={full?.installed ? "Installed" : "Not found on PATH"}
                />
                {a.name}
                {full && <CounterLabel>{full.extensions.length}</CounterLabel>}
              </button>
            );
          })}
        </nav>
        <div className="header-actions">
          <Select
            aria-label="Project scope"
            size="small"
            value={project}
            onChange={(e) =>
              e.target.value === "__custom"
                ? setCustomProject(true)
                : chooseProject(e.target.value)
            }
          >
            <Select.Option value="">No project (user settings)</Select.Option>
            {Array.from(
              new Set([
                ...(catalog?.projects || []),
                ...(project ? [project] : []),
              ]),
            ).map((p) => (
              <Select.Option key={p} value={p}>
                {tilde(p)}
              </Select.Option>
            ))}
            <Select.Option value="__custom">Other folder…</Select.Option>
          </Select>
          <IconButton
            icon={SyncIcon}
            size="small"
            aria-label="Reload from disk"
            onClick={() => {
              setReload((v) => v + 1);
              setNotice("Reloaded from disk.");
            }}
          />
        </div>
      </header>
      <main>
        <div className="page-title">
          <Heading as="h1" className="agent-heading">
            {agent?.name || "Agent Desk"}
          </Heading>
          {agent && (
            <Label variant={agent.installed ? "success" : "secondary"}>
              {agent.installed ? "Installed" : "Not on PATH"}
            </Label>
          )}
          {agent?.executable && (
            <code className="page-meta">{agent.executable}</code>
          )}
        </div>
        <UnderlineNav aria-label="Sections" className="sections">
          {tabs.map((t) => (
            <UnderlineNav.Item
              key={t}
              href={"#" + t.toLowerCase()}
              aria-current={t === tab ? "page" : undefined}
              icon={tabIcons[t]}
              counter={
                !agent || t === "Sessions"
                  ? undefined
                  : t === "Files"
                    ? fileCount
                    : agent.extensions.length
              }
              onSelect={(e) => {
                e.preventDefault();
                setTab(t);
                setSearch("");
                setKind("all");
              }}
            >
              {t}
            </UnderlineNav.Item>
          ))}
        </UnderlineNav>
        <div className="content">
          {error && !opened && (
            <div role="alert">
              <Banner
                variant="critical"
                title={error}
                layout="compact"
                onDismiss={() => setError("")}
              />
            </div>
          )}
          {notice && (
            <div role="status">
              <Banner
                variant="success"
                title={notice}
                layout="compact"
                onDismiss={() => setNotice("")}
              />
            </div>
          )}
          {agent?.warnings.map((w, i) => (
            <Banner key={i} variant="warning" title={w} layout="compact" />
          ))}
          <div className="toolbar">
            <TextInput
              ref={searchBox}
              className="search"
              type="search"
              aria-label="Search"
              leadingVisual={SearchIcon}
              trailingVisual={<KeybindingHint keys="/" />}
              placeholder={
                tab === "Sessions"
                  ? "Search titles and folders"
                  : tab === "Files"
                    ? "Search file paths"
                    : "Search names and paths"
              }
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {tab === "Extensions" && (
              <>
                <Select
                  aria-label="Extension type"
                  value={kind}
                  onChange={(e) => setKind(e.target.value)}
                >
                  <Select.Option value="all">All types</Select.Option>
                  {Object.entries(kindNames).map(([k, name]) => (
                    <Select.Option key={k} value={k}>
                      {name}s
                    </Select.Option>
                  ))}
                </Select>
                <Button
                  variant="primary"
                  leadingVisual={PlusIcon}
                  onClick={() => {
                    setSkillName("");
                    setSkillDescription("");
                    setNewSkill(true);
                  }}
                >
                  New skill
                </Button>
              </>
            )}
          </div>
          {tab === "Files" &&
            fileGroups.map((g) => (
              <section className="box" key={g.kind}>
                <h2 className="box-header">
                  {g.title}
                  <CounterLabel>{g.files.length}</CounterLabel>
                </h2>
                {g.files.length === 0 ? (
                  <p className="box-empty">No matching files.</p>
                ) : (
                  g.files.map((f) => (
                    <FileRow
                      key={f.id}
                      file={f}
                      path={tilde(f.path)}
                      selected={opened?.file.id === f.id}
                      onOpen={() => openFile(f.id)}
                    />
                  ))
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
                <Blankslate border>
                  <Blankslate.Visual>
                    <PlugIcon size="medium" />
                  </Blankslate.Visual>
                  <Blankslate.Heading>
                    No matching extensions
                  </Blankslate.Heading>
                  <Blankslate.Description>
                    Nothing was found in the supported locations.
                  </Blankslate.Description>
                </Blankslate>
              ) : (
                <section className="box">
                  {ext.map((x, i) => (
                    <button
                      className={
                        "row extension-row" +
                        (opened?.file.id === x.fileId ? " selected" : "")
                      }
                      key={x.fileId + x.name + i}
                      onClick={() => openFile(x.fileId)}
                    >
                      <span className="row-icon">
                        {(() => {
                          const Icon = kindIcons[x.kind] || FileIcon;
                          return <Icon />;
                        })()}
                      </span>
                      <span className="row-main">
                        <strong>{x.name}</strong>
                        <small title={x.source}>{tilde(x.source)}</small>
                      </span>
                      <span className="row-meta">
                        <Label variant="secondary">{kindNames[x.kind]}</Label>
                        <span className="row-scope">{x.scope}</span>
                        <span className={"row-status status-" + x.status}>
                          {x.status}
                        </span>
                      </span>
                    </button>
                  ))}
                </section>
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
                <Banner key={i} variant="warning" title={w} layout="compact" />
              ))}
              <div className="sessions-layout">
                <section className="box session-list">
                  {sessionBusy ? (
                    <div className="box-empty">
                      <Spinner size="small" /> Loading…
                    </div>
                  ) : visibleSessions.length === 0 ? (
                    <p className="box-empty">
                      No matching sessions. Try another project or clear your
                      search.
                    </p>
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
                          <RelativeTime date={new Date(s.updated)} />
                          {s.format === "indexed excerpts" ? " · Excerpts" : ""}
                        </small>
                      </button>
                    ))
                  )}
                </section>
                <section className="transcript">
                  {!session ? (
                    <Blankslate border>
                      <Blankslate.Visual>
                        <CommentDiscussionIcon size="medium" />
                      </Blankslate.Visual>
                      <Blankslate.Heading>
                        Pick a conversation
                      </Blankslate.Heading>
                      <Blankslate.Description>
                        Conversations are read-only.
                      </Blankslate.Description>
                    </Blankslate>
                  ) : (
                    <>
                      <div className="transcript-heading">
                        <Heading as="h2" className="transcript-title">
                          {session.title}
                        </Heading>
                        <TextInput
                          type="search"
                          aria-label="Find in conversation"
                          placeholder="Filter messages"
                          leadingVisual={SearchIcon}
                          value={transcriptSearch}
                          onChange={(e) => setTranscriptSearch(e.target.value)}
                        />
                      </div>
                      {!transcript ? (
                        <div className="box-empty">
                          <Spinner size="small" /> Loading…
                        </div>
                      ) : (
                        <>
                          <p className="footnote">
                            {transcript.note || "Stored conversation text."}
                          </p>
                          {transcript.messages.length === 0 && (
                            <p className="box-empty">
                              {transcriptQuery
                                ? "No matching messages."
                                : "This session has no text in a supported format."}
                            </p>
                          )}
                          {transcript.messages.map((m, i) => {
                            const copy = (
                              <IconButton
                                icon={CopyIcon}
                                size="small"
                                variant="invisible"
                                aria-label="Copy message"
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
                              />
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
                                <Button
                                  size="small"
                                  loading={moreBusy}
                                  onClick={loadMoreMessages}
                                >
                                  Show more
                                </Button>
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
            <div className="panel-title">
              <Heading as="h2">
                {opened?.file.name || "Loading…"}
                {dirty && <Label variant="attention">Unsaved</Label>}
              </Heading>
              {opened && (
                <p className="file-path" title={opened.file.path}>
                  <code>{tilde(opened.file.path)}</code>
                  <Label variant="secondary">{opened.file.scope}</Label>
                  {!opened.file.editable && (
                    <Label variant="secondary">
                      <LockIcon size={12} /> read-only
                    </Label>
                  )}
                </p>
              )}
            </div>
            <IconButton
              icon={XIcon}
              variant="invisible"
              disabled={busy}
              aria-label="Close editor"
              onClick={closeEditor}
            />
          </div>
          {opened && (
            <>
              {error && (
                <div role="alert" className="panel-banner">
                  <Banner variant="critical" title={error} layout="compact" />
                </div>
              )}
              {opened.file.target && (
                <p className="panel-note">
                  <LinkIcon size={12} /> This path is a link. Changes are saved
                  to <code>{tilde(opened.file.target)}</code>, and the link
                  stays in place.
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
                    dark={dark}
                  />
                )
              ) : review ? (
                <Changes
                  title={opened.file.exists ? "Changes to save" : "New file"}
                  before={opened.content}
                  after={draft}
                  language={languageFor(opened.file.name)}
                  dark={dark}
                />
              ) : (
                <div className="code-editor">
                  <CodeMirror
                    value={draft}
                    height="100%"
                    theme={dark ? "dark" : "light"}
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
                      <Button
                        variant="danger"
                        leadingVisual={TrashIcon}
                        disabled={busy}
                        onClick={remove}
                      >
                        {opened.file.kind === "skill"
                          ? "Remove skill"
                          : "Remove file"}
                      </Button>
                    )}
                  {opened.undo && !review && !undoReview && (
                    <Button
                      leadingVisual={HistoryIcon}
                      disabled={busy || dirty}
                      title={dirty ? "Save or discard your edits first" : ""}
                      onClick={() => setUndoReview(true)}
                    >
                      Undo last save
                    </Button>
                  )}
                </div>
                <div>
                  {undoReview && (
                    <>
                      <Button
                        disabled={busy}
                        onClick={() => setUndoReview(false)}
                      >
                        Back to editing
                      </Button>
                      <Button variant="primary" loading={busy} onClick={undo}>
                        Confirm undo
                      </Button>
                    </>
                  )}
                  {review && (
                    <Button disabled={busy} onClick={() => setReview(false)}>
                      Back to editing
                    </Button>
                  )}
                  {opened.file.editable && !undoReview && (
                    <Button
                      variant="primary"
                      disabled={!dirty}
                      loading={busy}
                      onClick={() => (review ? save() : setReview(true))}
                    >
                      {review ? "Confirm save" : "Review changes"}
                    </Button>
                  )}
                </div>
              </div>
            </>
          )}
        </section>
      )}
      {newSkill && (
        <Dialog
          title="New skill"
          subtitle={
            "For " +
            (agent?.name || "") +
            ", " +
            (project ? "in " + tilde(project) : "in your user settings")
          }
          onClose={() => setNewSkill(false)}
          footerButtons={[
            { content: "Cancel", onClick: () => setNewSkill(false) },
            {
              content: "Create and edit",
              buttonType: "primary",
              type: "submit",
              form: "new-skill",
              disabled: busy,
            },
          ]}
        >
          <form
            id="new-skill"
            className="dialog-form"
            onSubmit={(e) => {
              e.preventDefault();
              createSkill();
            }}
          >
            {error && (
              <Banner variant="critical" title={error} layout="compact" />
            )}
            <FormControl required>
              <FormControl.Label>Name</FormControl.Label>
              <TextInput
                block
                pattern="[a-z0-9][a-z0-9-]{0,63}"
                placeholder="review-changes"
                value={skillName}
                onChange={(e) => setSkillName(e.target.value)}
              />
              <FormControl.Caption>
                Lowercase letters, numbers, and hyphens.
              </FormControl.Caption>
            </FormControl>
            <FormControl required>
              <FormControl.Label>Description</FormControl.Label>
              <Textarea
                block
                placeholder="When should this skill be used?"
                value={skillDescription}
                onChange={(e) => setSkillDescription(e.target.value)}
              />
            </FormControl>
          </form>
        </Dialog>
      )}
      {customProject && (
        <Dialog
          title="Choose a project"
          onClose={() => setCustomProject(false)}
          footerButtons={[
            { content: "Cancel", onClick: () => setCustomProject(false) },
            {
              content: "Open project",
              buttonType: "primary",
              type: "submit",
              form: "choose-project",
            },
          ]}
        >
          <form
            id="choose-project"
            className="dialog-form"
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
            {error && (
              <Banner variant="critical" title={error} layout="compact" />
            )}
            <FormControl required>
              <FormControl.Label>Folder inside your home</FormControl.Label>
              <TextInput
                block
                placeholder="~/projects/my-project"
                value={projectDraft}
                onChange={(e) => setProjectDraft(e.target.value)}
              />
            </FormControl>
          </form>
        </Dialog>
      )}
    </div>
  );
}
