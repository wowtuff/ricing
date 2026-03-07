package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsClientMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type wsSubscribe struct {
	RunID    string `json:"run_id"`
	AfterSeq int64  `json:"after_seq"`
}

type wsCancel struct {
	RunID string `json:"run_id"`
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	send := make(chan any, 128)
	var writeMu sync.Mutex
	emit := func(v any) {
		select {
		case send <- v:
		default:
		}
	}
	writeJSONMsg := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	// initial hello
	_ = writeJSONMsg(map[string]any{
		"v":    1,
		"type": "server.hello",
		"data": map[string]any{
			"protocol": map[string]any{"v": 1},
			"capabilities": map[string]any{
				"subscribe": true,
			},
		},
	})

	// writer loop
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range send {
			_ = writeJSONMsg(msg)
		}
	}()

	var unsubscribe func()
	var subWG sync.WaitGroup

	readLoop := func() error {
		for {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return err
			}
			var msg wsClientMsg
			if err := json.Unmarshal(payload, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "ping":
				emit(map[string]any{"v": 1, "type": "pong"})

			case "subscribe":
				var sub wsSubscribe
				if err := json.Unmarshal(msg.Data, &sub); err != nil || sub.RunID == "" {
					emit(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "bad_subscribe", "message": "bad subscribe payload"}})
					continue
				}

				hub, ok := s.runs.Hub(sub.RunID)
				if !ok {
					emit(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "run_not_found", "message": "run not found"}})
					continue
				}

				if unsubscribe != nil {
					unsubscribe()
					unsubscribe = nil
					subWG.Wait()
				}

				// backlog
				for _, ev := range hub.ListAfter(sub.AfterSeq) {
					emit(ev)
				}

				_, ch, unsub := hub.Subscribe()
				unsubscribe = unsub
				subWG.Add(1)
				go func() {
					defer subWG.Done()
					for ev := range ch {
						emit(ev)
					}
				}()

				emit(map[string]any{"v": 1, "type": "subscribed", "data": map[string]any{"run_id": sub.RunID, "after_seq": sub.AfterSeq}})

			case "run.cancel":
				var c wsCancel
				if err := json.Unmarshal(msg.Data, &c); err != nil || c.RunID == "" {
					emit(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "bad_cancel", "message": "bad cancel payload"}})
					continue
				}
				_, _ = s.runs.Cancel(c.RunID)

			default:
				emit(map[string]any{"v": 1, "type": "error", "data": map[string]any{"code": "unknown_type", "message": "unknown message type"}})
			}
		}
	}

	_ = readLoop()

	if unsubscribe != nil {
		unsubscribe()
	}
	subWG.Wait()
	close(send)
	<-done
}
