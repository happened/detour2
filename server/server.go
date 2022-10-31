package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

type Server struct {
	Address  string
	Network  string
	Password string
	Handler  *Handler
}

func NewServer(listen string, password string) *Server {
	vals := strings.Split(listen, "://")
	network := vals[0]
	address := vals[1]
	server := &Server{Address: address, Network: network, Password: password}
	server.Handler = NewHandler(server)
	return server
}

func (s *Server) Run() {
	http.HandleFunc("/", s.HandleIndex)
	http.HandleFunc("/ws", s.HandleWebsocket)

	log.Printf("Listening at http://%s", s.Address)

	httpServer := &http.Server{
		Addr: s.Address,
	}

	// graceful shutdown
	idleConnectionsClosed := make(chan struct{})
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
		<-sigch
		if err := httpServer.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP Server Shutdown Error: %v", err)
		}
		close(idleConnectionsClosed)
	}()

	// periodically run tasks
	go func() {
		for {
			select {
			case <-idleConnectionsClosed:
				return
			default:
			}
			time.Sleep(time.Second * 10)
			s.Handler.Tracker.RunHouseKeeper()
		}
	}()

	// listen and block
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe Error: %v", err)
	}

	<-idleConnectionsClosed

	log.Printf("Bye bye")
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("HTTP %s %s%s\n", r.Method, r.Host, r.URL)

	if r.URL.Path != "/" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(time.Now().Local().Format(time.RFC3339)))
}

func (s *Server) HandleWebsocket(w http.ResponseWriter, r *http.Request) {

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()

		if err != nil {
			if !strings.Contains(err.Error(), "1006") {
				log.Println("read error:", err)
			}
			break
		}

		if mt == websocket.BinaryMessage {
			s.Handler.HandleRelay(message, c)
		} else if mt == websocket.CloseMessage {
			break
		}
	}
}
