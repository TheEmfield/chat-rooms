package wsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/TheEmfield/chat-rooms/internal/config"
	"github.com/gorilla/websocket"
)

const (
	templateDir = "./web/templates/html"
	staticDir   = "./web/static"
)

type WSServer interface {
	Start() error
	Stop() error
}

type room struct {
	id          string
	rmClients   map[*websocket.Conn]struct{}
	messages    []*wsMessage
	mutex       *sync.RWMutex
	broadcast   chan *wsMessage
	capacity    int
	maxMessages int
	logger      *slog.Logger
}

type roomInfo struct {
	ID       string `json:"id"`
	Clients  int    `json:"clients"`
	Capacity int    `json:"capacity"`
}

func newRoom(id string, capacity, maxMessages int, l *slog.Logger) *room {
	return &room{
		id:          id,
		rmClients:   map[*websocket.Conn]struct{}{},
		messages:    make([]*wsMessage, 0, maxMessages),
		mutex:       &sync.RWMutex{},
		broadcast:   make(chan *wsMessage),
		capacity:    capacity,
		maxMessages: maxMessages,
		logger:      l,
	}
}

func (r *room) isFull() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.rmClients) >= r.capacity
}

type wsSrv struct {
	srv     *http.Server
	mux     *http.ServeMux
	wsUpg   *websocket.Upgrader
	wsRooms map[string]*room
	mutex   *sync.RWMutex
	logger  *slog.Logger
}

func NewWsServer(cfg *config.Config, l *slog.Logger) WSServer {
	m := http.NewServeMux()

	rooms := make(map[string]*room)
	for i := 1; i <= cfg.HTTP.NumberRooms; i++ {
		id := fmt.Sprintf("room-%d", i)
		rooms[id] = newRoom(id, cfg.HTTP.NumberClients, cfg.HTTP.NumberMessages, l)
	}

	return &wsSrv{
		mux: m,
		srv: &http.Server{
			Addr:    cfg.HTTP.Host + ":" + cfg.HTTP.Port,
			Handler: m,
		},
		wsUpg:   &websocket.Upgrader{},
		wsRooms: rooms,
		mutex:   &sync.RWMutex{},
		logger:  l,
	}
}

func (ws *wsSrv) Start() error {
	ws.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	ws.mux.Handle("/", http.FileServer(http.Dir(templateDir)))
	ws.mux.HandleFunc("/ws", ws.wsHandler)
	ws.mux.HandleFunc("/api/rooms", ws.roomsHandler)
	go ws.writeToClientsBroadcast()
	return ws.srv.ListenAndServe()
}

func (ws *wsSrv) Stop() error {
	ws.mutex.RLock()
	defer ws.mutex.Unlock()
	for _, room := range ws.wsRooms {
		room.mutex.Lock()
		for conn := range room.rmClients {
			if err := conn.Close(); err != nil {
				ws.logger.Error("error with closing", "error", err)
			}
			delete(room.rmClients, conn)
		}
		room.mutex.Unlock()
	}
	ws.mutex.RUnlock()

	for _, room := range ws.wsRooms {
		close(room.broadcast)
	}

	return ws.srv.Shutdown(context.Background())
}

func (ws *wsSrv) roomsHandler(w http.ResponseWriter, r *http.Request) {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()

	rooms := make([]roomInfo, 0, len(ws.wsRooms))
	for id, room := range ws.wsRooms {
		room.mutex.RLock()
		clients := len(room.rmClients)
		room.mutex.RUnlock()

		rooms = append(rooms, roomInfo{
			ID:       id,
			Clients:  clients,
			Capacity: room.capacity,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func (ws *wsSrv) wsHandler(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "room parameter required", http.StatusBadRequest)
		return
	}

	ws.mutex.RLock()
	room, exists := ws.wsRooms[roomID]
	ws.mutex.RUnlock()

	if !exists {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	if room.isFull() {
		http.Error(w, "room is full", http.StatusServiceUnavailable)
		return
	}

	conn, err := ws.wsUpg.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error("error with websocket connection", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ws.logger.Info("new client", "addr", conn.RemoteAddr().String(), "room", roomID)

	room.mutex.Lock()
	room.rmClients[conn] = struct{}{}
	history := make([]*wsMessage, len(room.messages))
	copy(history, room.messages)
	room.mutex.Unlock()

	if len(history) > 0 {
		historyMsg := &wsMessage{
			Type:     "history",
			Messages: history,
		}
		conn.WriteJSON(historyMsg)
	}

	go room.readFromClient(conn)
}

func (r *room) readFromClient(conn *websocket.Conn) {
	for {
		msg := new(wsMessage)
		if err := conn.ReadJSON(msg); err != nil {
			rmErr, ok := err.(*websocket.CloseError)
			if !ok || rmErr.Code != websocket.CloseGoingAway {
				r.logger.Error("error with reading from WebSocket", "error", err)
			}
			break
		}
		host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			r.logger.Error("error with address split", "error", err)
		}
		msg.IPAddress = host
		msg.Time = time.Now().Format("15:04")

		r.mutex.Lock()
		r.messages = append(r.messages, msg)
		if len(r.messages) > r.maxMessages {
			r.messages = r.messages[len(r.messages)-r.maxMessages:]
		}
		r.mutex.Unlock()

		r.broadcast <- msg
	}
	r.mutex.Lock()
	delete(r.rmClients, conn)
	r.mutex.Unlock()
}

func (ws *wsSrv) writeToClientsBroadcast() {
	for _, room := range ws.wsRooms {
		go ws.broadcastRoom(room)
	}
}

func (ws *wsSrv) broadcastRoom(r *room) {
	for msg := range r.broadcast {
		r.mutex.RLock()
		for client := range r.rmClients {
			if err := client.WriteJSON(msg); err != nil {
				ws.logger.Error("error with writing message", "error", err)
			}
		}
		r.mutex.RUnlock()
	}
}
