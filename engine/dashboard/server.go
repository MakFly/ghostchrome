package dashboard

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

//go:embed static
var staticFiles embed.FS

type server struct {
	httpSrv  *http.Server
	listener net.Listener
}

func newServer(port int, str *streamer) (*server, string, error) {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, "", err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	mux.Handle("/ws", websocket.Handler(func(conn *websocket.Conn) {
		ch := str.subscribe()
		defer str.unsubscribe(ch)
		for data := range ch {
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}))

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, "", err
	}
	addr := fmt.Sprintf("http://%s", ln.Addr().String())
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return &server{httpSrv: srv, listener: ln}, addr, nil
}

func (s *server) serve(ctx context.Context) {
	go func() {
		<-ctx.Done()
		_ = s.httpSrv.Close()
	}()
	_ = s.httpSrv.Serve(s.listener)
}
