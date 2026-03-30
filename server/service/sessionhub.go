package service

import (
	"sync"
	"time"
)

type SessionHub struct {
	sessionID string
	capacity  int
	mu        sync.Mutex
	nextSeq   int64
	events    []Event
	subs      map[string]chan Event
}

func NewSessionHub(sessionID string, capacity int) *SessionHub {
	if capacity <= 0 {
		capacity = 2048
	}
	return &SessionHub{
		sessionID: sessionID,
		capacity:  capacity,
		events:    make([]Event, 0, capacity),
		subs:      make(map[string]chan Event),
	}
}

func (h *SessionHub) Publish(typ string, data any) Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextSeq++
	ev := Event{
		V:         1,
		Type:      typ,
		SessionID: h.sessionID,
		Seq:       h.nextSeq,
		Time:      time.Now().UTC().Format(time.RFC3339Nano),
		Data:      data,
	}
	h.events = append(h.events, ev)
	if len(h.events) > h.capacity {
		h.events = append([]Event(nil), h.events[len(h.events)-h.capacity:]...)
	}
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	return ev
}

func (h *SessionHub) SetNextSeq(next int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if next > h.nextSeq {
		h.nextSeq = next
	}
}

func (h *SessionHub) ListAfter(afterSeq int64) []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) == 0 {
		return nil
	}
	out := make([]Event, 0, len(h.events))
	for _, ev := range h.events {
		if ev.Seq > afterSeq {
			out = append(out, ev)
		}
	}
	return out
}

func (h *SessionHub) Subscribe() (string, <-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subID := newID("sub")
	ch := make(chan Event, 256)
	h.subs[subID] = ch
	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if existing, ok := h.subs[subID]; ok {
			delete(h.subs, subID)
			close(existing)
		}
	}
	return subID, ch, unsubscribe
}
