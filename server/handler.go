package server

import (
	"detour/relay"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const DOWNSTREAM_BUFSIZE = 32 * 1024
const CONNECT_TIMEOUT = 3
const READWRITE_TIMEOUT = 60

type Handler struct {
	Tracker *Tracker
	Server  *Server
}

func NewHandler(server *Server) *Handler {
	return &Handler{Tracker: NewTracker(), Server: server}
}

func (h *Handler) HandleRelay(msg *relay.RelayMessage, writer chan *relay.RelayMessage) {
	switch msg.Data.CMD {
	case relay.CONNECT:
		h.handleConnect(msg, writer)
	case relay.DATA:
		h.handleData(msg, writer)
	default:
		log.Println("cmd not supported:", msg.Data.CMD)
		writer <- nil
	}
}

func (h *Handler) handleConnect(msg *relay.RelayMessage, writer chan *relay.RelayMessage) {
	// do connect
	log.Println("connect:", msg.Data.Address)

	conn, err := createConnection(msg.Data, writer)
	if err != nil {
		log.Println("connect error:", err)
		writer <- newErrorMessage(msg, err)
		return
	}

	// make response
	writer <- newOKMessage(msg)

	// connect success, update tracker
	h.Tracker.Upsert(msg.Pair, conn)

	// pull data from remote => local
	go h.runPuller(msg, conn, writer)
}

func (h *Handler) runPuller(msg *relay.RelayMessage, conn *relay.ConnInfo, writer chan *relay.RelayMessage) {
	defer func() {
		log.Println("stopped pulling:", conn.Address)
		// h.Tracker.Remove(msg.Pair)
		if conn.RemoteConn != nil {
			conn.RemoteConn.Close()
		}
	}()

	buf := make([]byte, DOWNSTREAM_BUFSIZE)
	log.Println("start pulling:", conn.Address)

	for {
		// conn.RemoteConn.SetReadDeadline(time.Now().Add(time.Second * READWRITE_TIMEOUT))
		nr, err := conn.RemoteConn.Read(buf)
		if err != nil && err != io.EOF {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Println("pull remote error:", err)
			}
			break
		}

		writer <- newDataMessage(msg, append([]byte{}, buf[0:nr]...))
		log.Println("remote => local data:", nr)
		if nr == 0 {
			log.Println("return on 0")
			break
		}
		// keep alive
		// h.Tracker.ImAlive(msg.Pair)
	}
}

func (h *Handler) handleData(msg *relay.RelayMessage, writer chan *relay.RelayMessage) {
	// find conn by tracker
	conn := h.Tracker.Find(msg.Pair)

	// conn is closed
	if conn == nil || conn.RemoteConn == nil {
		// tell the local to reconnect
		writer <- newReconnectMessage(msg)
		return
	}

	h.Tracker.ImAlive(msg.Pair)

	// push data => remote
	conn.RemoteConn.SetWriteDeadline(time.Now().Add(time.Second * READWRITE_TIMEOUT))
	n, err := conn.RemoteConn.Write(msg.Data.Data)
	if err != nil {
		log.Println("write error:", err)
		conn.RemoteConn.Close()
		return
	}

	log.Println("local => remote data:", n)
	if n == 0 {
		conn.RemoteConn.Close()
	}
}

func createConnection(req *relay.RelayData, writer chan *relay.RelayMessage) (*relay.ConnInfo, error) {
	conn, err := net.DialTimeout(req.Network, req.Address, time.Second*CONNECT_TIMEOUT)
	if err != nil {
		return nil, err
	}
	return &relay.ConnInfo{
		Network:    req.Network,
		Address:    req.Address,
		Activity:   time.Now().UnixMilli(),
		RemoteConn: conn,
		Writer:     writer,
	}, nil
}

// func (h *Handler) writeMessage(c *websocket.Conn, msg *relay.RelayMessage) error {
// 	// log.Println("write:", msg.Data)
// 	lock.Lock()
// 	defer lock.Unlock()
// 	return c.WriteMessage(websocket.BinaryMessage, relay.Pack(msg, h.Server.Password))
// }

func newErrorMessage(msg *relay.RelayMessage, err error) *relay.RelayMessage {
	return &relay.RelayMessage{
		Pair: msg.Pair,
		Data: &relay.RelayData{CMD: relay.CONNECT, OK: false, MSG: err.Error()},
	}
}

func newOKMessage(msg *relay.RelayMessage) *relay.RelayMessage {
	return &relay.RelayMessage{
		Pair: msg.Pair,
		Data: &relay.RelayData{CMD: relay.CONNECT, OK: true},
	}
}

// func newSwitchMessage(msg *relay.RelayMessage) *relay.RelayMessage {
// 	return &relay.RelayMessage{
// 		Pair: msg.Pair,
// 		Data: &relay.RelayData{CMD: relay.SWITCH, OK: true},
// 	}
// }

func newReconnectMessage(msg *relay.RelayMessage) *relay.RelayMessage {
	return &relay.RelayMessage{
		Pair: msg.Pair,
		Data: &relay.RelayData{CMD: relay.RECONNECT, OK: true},
	}
}

func newDataMessage(msg *relay.RelayMessage, data []byte) *relay.RelayMessage {
	return &relay.RelayMessage{
		Pair: msg.Pair,
		Data: &relay.RelayData{CMD: relay.DATA, Data: data},
	}
}
