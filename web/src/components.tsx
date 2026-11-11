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
