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
}

func New(home string) *Server {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return &Server{Home: home, Token: hex.EncodeToString(b), undo: map[string]undoRecord{}}
}
