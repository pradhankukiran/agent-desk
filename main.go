package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"github.com/pradhankukiran/agent-desk/internal/desk"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

//go:embed all:web/dist
var ui embed.FS

func main() {
	port := flag.Int("port", 7437, "local port (0 chooses a free port)")
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	homeFlag := flag.String("home", "", "agent home directory (defaults to your home)")
	flag.Parse()
	home := *homeFlag
	if home == "" {
		var e error
		home, e = os.UserHomeDir()
		if e != nil {
			log.Fatal(e)
		}
	}
	s := desk.New(home)
	assets, e := fs.Sub(ui, "web/dist")
	if e != nil {
		log.Fatal(e)
	}
	listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if e != nil {
		log.Fatal(e)
	}
	link := fmt.Sprintf("http://localhost:%d/#token=%s", listener.Addr().(*net.TCPAddr).Port, s.Token)
	fmt.Printf("Agent Desk\n%s\nPress Ctrl+C to stop.\n", link)
	server := &http.Server{Handler: s.Handler(assets), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			for _, cmd := range [][]string{{"wslview", link}, {"wsl-browser", link}, {"xdg-open", link}} {
				if _, e := exec.LookPath(cmd[0]); e == nil {
					if e := exec.Command(cmd[0], cmd[1:]...).Run(); e == nil {
						return
					}
				}
			}
			fmt.Println("Open the link above in your browser.")
		}()
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	if e = server.Serve(listener); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
