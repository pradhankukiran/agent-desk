export type FileItem = {
  id: string;
  agent: string;
  name: string;
  path: string;
  kind: string;
  scope: string;
  exists: boolean;
  editable: boolean;
  size: number;
  target?: string;
};
export type Extension = {
  name: string;
  kind: string;
  status: string;
  scope: string;
  fileId: string;
  source: string;
};
export type Agent = {
  id: string;
  name: string;
  installed: boolean;
  executable: string;
  files: FileItem[];
  extensions: Extension[];
  warnings: string[];
};
export type Catalog = { agents: Agent[]; projects: string[]; home: string };
export type OpenFile = {
  file: FileItem;
  content: string;
  revision: string;
  skillFiles?: number;
  skillProblem?: string;
  undo?: { content: string; removes: boolean };
};
export type Session = {
  id: string;
  agent: string;
  title: string;
  project: string;
  updated: string;
  path: string;
  format: string;
};
export type Transcript = {
  messages: { role: string; text: string; time?: string }[];
  note: string;
  truncated: boolean;
  total: number;
  offset: number;
};
