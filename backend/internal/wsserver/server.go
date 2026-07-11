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

	"github.com/TheEmfield/chat-rooms/backend/internal/config"
	"github.com/TheEmfield/chat-rooms/backend/internal/repository/entity"
	"github.com/gorilla/websocket"
)

const (
	templateDir = "./web/templates/html"
	staticDir   = "./web/static"
)

type storage interface {
	UpsertRoom(ctx context.Context, room entity.Room) error
	GetRoom(ctx context.Context, id string) (*entity.Room, error)
	UpsertMessage(ctx context.Context, msg entity.Message) error
	GetHistory(ctx context.Context, roomID string, limit int) ([]*entity.Message, error)
}

type WSServer interface {
	Start() error
	Stop(ctx context.Context) error
}

type room struct {
	id        string
	rmClients map[*websocket.Conn]struct{}
	mutex     *sync.RWMutex
	broadcast chan *wsMessage
	capacity  int
	logger    *slog.Logger
}

type roomInfo struct {
	ID       string `json:"id"`
	Clients  int    `json:"clients"`
	Capacity int    `json:"capacity"`
}

func newRoom(id string, capacity int, l *slog.Logger) *room {
	return &room{
		id:        id,
		rmClients: map[*websocket.Conn]struct{}{},
		mutex:     &sync.RWMutex{},
		broadcast: make(chan *wsMessage),
		capacity:  capacity,
		logger:    l,
	}
}

func (r *room) isFull() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.rmClients) >= r.capacity
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
		r.broadcast <- msg
	}
	r.mutex.Lock()
	delete(r.rmClients, conn)
	r.mutex.Unlock()
}

type wsSrv struct {
	srv         *http.Server
	mux         *http.ServeMux
	wsUpg       *websocket.Upgrader
	wsRooms     map[string]*room
	mutex       *sync.RWMutex
	logger      *slog.Logger
	wgRead      *sync.WaitGroup
	wgBroadcast *sync.WaitGroup
	storage     storage
	maxMessages int
}

func NewWsServer(cfg *config.Config, l *slog.Logger, st storage) WSServer {
	m := http.NewServeMux()

	rooms := make(map[string]*room)
	for i := 1; i <= cfg.HTTP.NumberRooms; i++ {
		id := fmt.Sprintf("room-%d", i)
		rooms[id] = newRoom(id, cfg.HTTP.NumberClients, l)
	}

	return &wsSrv{
		mux: m,
		srv: &http.Server{
			Addr:    cfg.HTTP.Host + ":" + cfg.HTTP.Port,
			Handler: m,
		},
		wsUpg:       &websocket.Upgrader{},
		wsRooms:     rooms,
		mutex:       &sync.RWMutex{},
		logger:      l,
		wgRead:      &sync.WaitGroup{},
		wgBroadcast: &sync.WaitGroup{},
		storage:     st,
		maxMessages: cfg.HTTP.NumberMessages,
	}
}

func (ws *wsSrv) Start() error {
	ws.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	ws.mux.Handle("/", http.FileServer(http.Dir(templateDir)))
	ws.mux.HandleFunc("/ws", ws.wsHandler)
	ws.mux.HandleFunc("/api/rooms", ws.roomsHandler)
	for _, room_ := range ws.wsRooms {
		ws.wgBroadcast.Add(1)
		go func(r *room) {
			defer ws.wgBroadcast.Done()
			ws.broadcastRoom(r)
		}(room_)
	}
	return ws.srv.ListenAndServe()
}

func (ws *wsSrv) Stop(ctx context.Context) error {
	if err := ws.srv.Shutdown(ctx); err != nil {
		ws.logger.Error("HTTP shutdown error", "error", err)
	}

	ws.mutex.Lock()
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
	ws.mutex.Unlock()

	ws.wgRead.Wait()

	for _, room := range ws.wsRooms {
		close(room.broadcast)
	}

	ws.wgBroadcast.Wait()

	return nil
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
	room.mutex.Unlock()
	ctx := context.Background()
	historyEntities, err := ws.storage.GetHistory(ctx, roomID, ws.maxMessages)
	if err != nil {
		ws.logger.Error("error loading history from DB", "error", err)
		historyEntities = make([]*entity.Message, 0)
	}

	if len(historyEntities) > 0 {
		history := make([]*wsMessage, 0, len(historyEntities))
		for _, e := range historyEntities {
			history = append(history, &wsMessage{
				IPAddress: e.SenderIP,
				Message:   e.Msg,
				Time:      e.CreatedAt.Format("15:04"),
			})
		}
		historyMsg := &wsMessage{
			Type:     "history",
			Messages: history,
		}
		conn.WriteJSON(historyMsg)
	}

	ws.wgRead.Add(1)
	go func() {
		defer ws.wgRead.Done()
		room.readFromClient(conn)
	}()
}

func (ws *wsSrv) broadcastRoom(r *room) {
	for msg := range r.broadcast {
		ctx := context.Background()
		dbMsg := entity.Message{
			RoomID:   r.id,
			SenderIP: msg.IPAddress,
			Msg:      msg.Message,
		}
		if err := ws.storage.UpsertMessage(ctx, dbMsg); err != nil {
			ws.logger.Error("error saving message to DB", "error", err)
		}

		r.mutex.RLock()
		for client := range r.rmClients {
			if err := client.WriteJSON(msg); err != nil {
				ws.logger.Error("error with writing message", "error", err)
			}
		}
		r.mutex.RUnlock()
	}
}
