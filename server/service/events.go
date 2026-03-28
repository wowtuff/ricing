package service

import (
	"sync"
	"time"
)

type Event struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Seq       int64  `json:"seq"`
	Time      string `json:"time"`
	Data      any    `json:"data,omitempty"`
}

type RunHub struct {
	runID    string
	capacity int

	mu      sync.Mutex
	nextSeq int64
	events  []Event
	subs    map[string]chan Event
}

func NewRunHub(runID string, capacity int) *RunHub {
	if capacity <= 0 {
		capacity = 2048
	}
	return &RunHub{
		runID:    runID,
		capacity: capacity,
		events:   make([]Event, 0, capacity),
		subs:     make(map[string]chan Event),
	}
}

func (h *RunHub) Publish(typ string, data any) Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextSeq++
	ev := Event{
		V:     1,
		Type:  typ,
		RunID: h.runID,
		Seq:   h.nextSeq,
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Data:  data,
	}

	h.events = append(h.events, ev)
	if len(h.events) > h.capacity {
		// drop oldest
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

func (h *RunHub) ListAfter(afterSeq int64) []Event {
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

func (h *RunHub) Subscribe() (subID string, ch <-chan Event, unsubscribe func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subID = newID("sub")
	c := make(chan Event, 256)
	h.subs[subID] = c

	unsubscribe = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if ch2, ok := h.subs[subID]; ok {
			delete(h.subs, subID)
			close(ch2)
		}
	}

	return subID, c, unsubscribe
}
