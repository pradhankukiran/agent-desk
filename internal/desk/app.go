package desk

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Server struct {
	Home, Token string
	mu          sync.Mutex
}

func New(home string) *Server {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return &Server{Home: home, Token: hex.EncodeToString(b)}
}
