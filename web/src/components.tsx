import { useEffect, useState } from "react";
import { BookIcon, GearIcon, LockIcon } from "@primer/octicons-react";
import CodeMirror from "@uiw/react-codemirror";
import type { Extension } from "@codemirror/state";
import { Text } from "@codemirror/state";
import { Chunk, unifiedMergeView } from "@codemirror/merge";
import type { FileItem } from "./types";

export function FileRow({
  file: f,
  path,
  selected,
  onOpen,
}: {
  file: FileItem;
  path: string;
  selected: boolean;
  onOpen: () => void;
}) {
  const Icon = f.kind === "instruction" ? BookIcon : GearIcon;
  return (
    <button
      className={"row file-row" + (selected ? " selected" : "")}
      onClick={onOpen}
    >
      <span className="row-icon">
        <Icon />
      </span>
      <span className="row-main">
        <strong className={f.exists ? "" : "missing"}>{f.name}</strong>
        <small title={f.path}>{path}</small>
      </span>
      <span className="row-meta">
        {!f.editable && <LockIcon size={14} aria-label="read-only" />}
        <span className="row-scope">{f.scope}</span>
        <span className="row-status">
          {f.exists ? formatSize(f.size) : "not created"}
        </span>
      </span>
    </button>
  );
}

// useDarkMode follows the system colour scheme, like the Primer theme.
export function useDarkMode() {
  const query = "(prefers-color-scheme: dark)";
  const [dark, setDark] = useState(() => matchMedia(query).matches);
  useEffect(() => {
    const m = matchMedia(query);
    const update = () => setDark(m.matches);
    m.addEventListener("change", update);
    return () => m.removeEventListener("change", update);
  }, []);
  return dark;
}

function formatSize(n: number) {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  return (n / 1024 / 1024).toFixed(1) + " MB";
}

// Changes shows a line-by-line comparison: removed lines in red, added
// lines in green, and long unchanged stretches folded away.
export function Changes({
  title,
  before,
  after,
  language,
  dark,
}: {
  title: string;
  before: string;
  after: string;
  language: Extension[];
  dark: boolean;
}) {
  const sections = Chunk.build(
    Text.of(before.split("\n")),
    Text.of(after.split("\n")),
  ).length;
  return (
    <div className="review-diff">
      <div className="review-summary">
        <h3>{title}</h3>
        <span>
          {sections === 0
            ? "No differences"
            : sections === 1
              ? "1 changed section"
              : `${sections} changed sections`}
        </span>
      </div>
      <CodeMirror
        value={after}
        height="100%"
        theme={dark ? "dark" : "light"}
        editable={false}
        basicSetup={{
          lineNumbers: true,
          foldGutter: false,
          highlightActiveLine: false,
        }}
        extensions={[
          ...language,
          unifiedMergeView({
            original: before,
            mergeControls: false,
            collapseUnchanged: { margin: 3, minSize: 6 },
          }),
        ]}
      />
    </div>
  );
}

const longMessage = 20000;

// MessageText shows a message, shortening very long ones until expanded so a
// single huge entry does not freeze the page.
export function MessageText({ text }: { text: string }) {
  const [full, setFull] = useState(false);
  if (full || text.length <= longMessage) return <pre>{text}</pre>;
  return (
    <>
      <pre>{text.slice(0, longMessage)}…</pre>
      <button className="text-button" onClick={() => setFull(true)}>
        Show full message ({text.length.toLocaleString()} characters)
      </button>
    </>
  );
}
