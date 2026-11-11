package desk

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestAPIAuthorizationAndOrigin(t *testing.T) {
	s := fixture(t)
	handler := s.Handler(fstest.MapFS{"index.html": {Data: []byte("test")}})
	for _, tc := range []struct {
		name, host, token, origin string
		want                      int
	}{{"authorized", "localhost:7437", s.Token, "", 200}, {"no token", "localhost:7437", "", "", 401}, {"wrong token", "localhost:7437", "wrong", "", 401}, {"foreign origin", "localhost:7437", s.Token, "https://evil.example", 403}, {"foreign host", "evil.example:7437", s.Token, "", 403}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+tc.host+"/api/catalog", nil)
			r.Header.Set("Authorization", "Bearer "+tc.token)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
	r := httptest.NewRequest("GET", "http://localhost:7437/api/file?id=../../etc/passwd", nil)
	r.Header.Set("Authorization", "Bearer "+s.Token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("unknown file accepted")
	}
}
