package desk

type File struct {
	ID       string `json:"id"`
	Agent    string `json:"agent"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Scope    string `json:"scope"`
	Exists   bool   `json:"exists"`
	Editable bool   `json:"editable"`
	Size     int64  `json:"size"`
}

type Extension struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Scope  string `json:"scope"`
	FileID string `json:"fileId"`
	Source string `json:"source"`
}

type Agent struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Executable string      `json:"executable"`
	Installed  bool        `json:"installed"`
	Files      []File      `json:"files"`
	Extensions []Extension `json:"extensions"`
	Warnings   []string    `json:"warnings"`
}

type Session struct {
	ID      string `json:"id"`
	Agent   string `json:"agent"`
	Title   string `json:"title"`
	Project string `json:"project"`
	Updated string `json:"updated"`
	Path    string `json:"path"`
	Format  string `json:"format"`
}

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
	Time string `json:"time,omitempty"`
}

type Transcript struct {
	Messages  []Message `json:"messages"`
	Note      string    `json:"note"`
	Truncated bool      `json:"truncated"`
}
