package desk

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Server struct {
	Home, Token string
	mu          sync.Mutex
	undo        map[string]undoRecord // last save per file path, guarded by mu

	cacheMu      sync.Mutex
	sessionCache map[string]sessionFile     // session list details per history file
	fileIndex    map[string]map[string]File // catalog files by project, then file ID
	scans        int                        // catalog scans so far, for tests
}

func New(home string) *Server {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return &Server{Home: home, Token: hex.EncodeToString(b), undo: map[string]undoRecord{}}
}
