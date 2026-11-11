import CodeMirror from "@uiw/react-codemirror";
import type { Extension } from "@codemirror/state";
import { Text } from "@codemirror/state";
import { Chunk, unifiedMergeView } from "@codemirror/merge";
import type { FileItem } from "./types";

export function FileRow({
  file: f,
  onOpen,
}: {
  file: FileItem;
  onOpen: () => void;
}) {
  return (
    <button className="file-row" onClick={onOpen}>
      <span className="file-icon">{f.kind === "instruction" ? "≡" : "⌘"}</span>
      <span className="file-info">
        <strong>{f.name}</strong>
        <small title={f.path}>{f.path}</small>
      </span>
      <span className="scope-chip">{f.scope}</span>
      <span className={"badge " + (f.exists ? "present" : "missing")}>
        {f.exists ? "Present" : "Not created"}
      </span>
      <span className="row-arrow">↗</span>
    </button>
  );
}
export function Empty({ title, text }: { title: string; text: string }) {
  return (
    <div className="empty">
      <span>◇</span>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}

// Changes shows a line-by-line comparison: removed lines in red, added
// lines in green, and long unchanged stretches folded away.
export function Changes({
  title,
  before,
  after,
  language,
}: {
  title: string;
  before: string;
  after: string;
  language: Extension[];
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
