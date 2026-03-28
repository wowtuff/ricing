package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const version = "0.1.0"
const serverURL = "http://127.0.0.1:1777"

type providerList struct {
	DefaultProviderID string `json:"default_provider_id"`
	Providers         []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	} `json:"providers"`
}

type session struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Mode             string `json:"mode"`
	Status           string `json:"status"`
	LatestPreview    string `json:"latest_preview"`
	PendingApprovals int    `json:"pending_approvals"`
	UpdatedAt        string `json:"updated_at"`
}

type entry struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Role      string         `json:"role"`
	Status    string         `json:"status"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Meta      map[string]any `json:"meta"`
}

type approval struct {
	ID       string `json:"id"`
	ToolName string `json:"tool_name"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

type attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type sessionSnapshot struct {
	Session     session      `json:"session"`
	Entries     []entry      `json:"entries"`
	Attachments []attachment `json:"attachments"`
	Approvals   []approval   `json:"approvals"`
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type client struct {
	mu                sync.Mutex
	sessionID         string
	runID             string
	ws                *websocket.Conn
	streamingEntryID  string
	defaultProviderID string
	connected         bool
	snapshot          *sessionSnapshot
}

func main() {
	fmt.Printf("ricing %s\n\n", version)
	c := &client{}
	if !checkServer() {
		fmt.Println("server not running. start with 'go run cmd/ricingd/main.go --ui-dir cmd/server/ui'")
		return
	}
	c.refreshProvider()
	c.loadInitialSession()
	c.printHelp()
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if c.handleCommand(line) {
				return
			}
			continue
		}
		if err := c.sendPrompt(line); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}
}

func checkServer() bool {
	resp, err := http.Get(serverURL + "/api/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *client) handleCommand(line string) bool {
	parts := strings.Fields(line)
	switch parts[0] {
	case "/quit", "/exit", "/q":
		c.closeSocket()
		fmt.Println("bye")
		return true
	case "/help", "/h", "/?":
		c.printHelp()
	case "/status":
		c.printStatus()
	case "/connect":
		if err := c.connectProvider(); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/sessions":
		if err := c.listSessions(); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/new":
		if err := c.createSession(); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/use":
		if len(parts) < 2 {
			fmt.Println("usage: /use <session_id>")
			return false
		}
		if err := c.selectSession(parts[1]); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/mode":
		if len(parts) < 2 {
			fmt.Println("usage: /mode <auto|plan|build>")
			return false
		}
		if err := c.setMode(parts[1]); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/approve":
		if len(parts) < 2 {
			fmt.Println("usage: /approve <approval_id>")
			return false
		}
		if err := c.resolveApproval(parts[1], "approve"); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/reject":
		if len(parts) < 2 {
			fmt.Println("usage: /reject <approval_id>")
			return false
		}
		if err := c.resolveApproval(parts[1], "reject"); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/files":
		c.printFiles()
	case "/attach":
		if len(parts) < 2 {
			fmt.Println("usage: /attach <file_path>")
			return false
		}
		if err := c.attachFile(strings.Join(parts[1:], " ")); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	case "/stop":
		if err := c.stopRun(); err != nil {
			fmt.Printf("error: %v\n", err)
		}
	default:
		fmt.Println("unknown command. use /help")
	}
	return false
}

func (c *client) printHelp() {
	fmt.Println("commands:")
	fmt.Println("  /connect           connect the default provider")
	fmt.Println("  /status            show provider and active session")
	fmt.Println("  /sessions          list sessions")
	fmt.Println("  /new               create a new session")
	fmt.Println("  /use <id>          switch to a session")
	fmt.Println("  /mode <mode>       set auto, plan, or build")
	fmt.Println("  /approve <id>      approve a pending action")
	fmt.Println("  /reject <id>       reject a pending action")
	fmt.Println("  /files             list files on the active session")
	fmt.Println("  /attach <path>     add a file to the active session")
	fmt.Println("  /stop              cancel the active run")
	fmt.Println("  /quit              exit")
	fmt.Println()
}

func (c *client) printStatus() {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := "disconnected"
	if c.connected {
		state = "connected"
	}
	active := "none"
	mode := "auto"
	if c.snapshot != nil {
		active = c.snapshot.Session.ID
		mode = c.snapshot.Session.Mode
	}
	fmt.Printf("provider: %s | session: %s | mode: %s\n", state, active, mode)
}

func (c *client) refreshProvider() {
	resp, err := http.Get(serverURL + "/api/v1/providers")
	if err != nil {
		c.connected = false
		return
	}
	defer resp.Body.Close()
	var data providerList
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.connected = false
		return
	}
	c.defaultProviderID = data.DefaultProviderID
	c.connected = false
	for _, provider := range data.Providers {
		if provider.ID == data.DefaultProviderID && provider.State == "connected" {
			c.connected = true
			return
		}
	}
}

func (c *client) connectProvider() error {
	c.refreshProvider()
	if c.connected {
		fmt.Println("provider already connected")
		return nil
	}
	body := bytes.NewBufferString(`{"open_browser":"server"}`)
	resp, err := http.Post(serverURL+"/api/v1/providers/"+c.defaultProviderID+"/connect", "application/json", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	fmt.Println("waiting for provider authentication...")
	started := time.Now()
	for time.Since(started) < 3*time.Minute {
		time.Sleep(2 * time.Second)
		c.refreshProvider()
		if c.connected {
			fmt.Println("provider connected")
			return nil
		}
	}
	return fmt.Errorf("provider connection timed out")
}

func (c *client) loadInitialSession() {
	resp, err := http.Get(serverURL + "/api/v1/sessions")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var data struct {
		Sessions []session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}
	if len(data.Sessions) == 0 {
		return
	}
	_ = c.selectSession(data.Sessions[0].ID)
}

func (c *client) listSessions() error {
	resp, err := http.Get(serverURL + "/api/v1/sessions")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var data struct {
		Sessions []session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	activeID := c.sessionID
	if len(data.Sessions) == 0 {
		fmt.Println("no sessions yet")
		return nil
	}
	for _, item := range data.Sessions {
		active := " "
		if item.ID == activeID {
			active = "*"
		}
		fmt.Printf("%s %s | %s | %s | %s\n", active, item.ID, item.Mode, item.Status, item.Title)
	}
	return nil
}

func (c *client) createSession() error {
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/api/v1/sessions", bytes.NewBufferString(`{"mode":"auto"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var data struct {
		Session session `json:"session"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	return c.selectSession(data.Session.ID)
}

func (c *client) selectSession(sessionID string) error {
	resp, err := http.Get(serverURL + "/api/v1/sessions/" + sessionID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var snapshot sessionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return err
	}
	c.mu.Lock()
	c.sessionID = sessionID
	c.snapshot = &snapshot
	c.streamingEntryID = ""
	c.mu.Unlock()
	c.openSocket(sessionID)
	c.printSnapshot(snapshot)
	return nil
}

func (c *client) setMode(mode string) error {
	if c.sessionID == "" {
		return fmt.Errorf("no active session")
	}
	body, _ := json.Marshal(map[string]string{"mode": mode})
	resp, err := http.Post(serverURL+"/api/v1/sessions/"+c.sessionID+"/mode", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeAPIError(resp.Body)
	}
	var data struct {
		Session session `json:"session"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	c.mu.Lock()
	if c.snapshot != nil {
		c.snapshot.Session = data.Session
	}
	c.mu.Unlock()
	fmt.Printf("mode set to %s\n", data.Session.Mode)
	return nil
}

func (c *client) sendPrompt(prompt string) error {
	if c.sessionID == "" {
		if err := c.createSession(); err != nil {
			return err
		}
	}
	body, _ := json.Marshal(map[string]any{
		"prompt": prompt,
		"llm": map[string]any{
			"provider_id": c.defaultProviderID,
		},
	})
	resp, err := http.Post(serverURL+"/api/v1/sessions/"+c.sessionID+"/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeAPIError(resp.Body)
	}
	var data struct {
		Run struct {
			ID string `json:"id"`
		} `json:"run"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	c.mu.Lock()
	c.runID = data.Run.ID
	c.mu.Unlock()
	return nil
}

func (c *client) resolveApproval(approvalID, decision string) error {
	body, _ := json.Marshal(map[string]string{"decision": decision})
	resp, err := http.Post(serverURL+"/api/v1/approvals/"+approvalID, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeAPIError(resp.Body)
	}
	fmt.Printf("%s %s\n", decision, approvalID)
	return nil
}

func (c *client) printFiles() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot == nil || len(c.snapshot.Attachments) == 0 {
		fmt.Println("no files attached")
		return
	}
	for _, item := range c.snapshot.Attachments {
		fmt.Printf("%s (%d bytes)\n", item.Name, item.Size)
	}
}

func (c *client) attachFile(path string) error {
	if c.sessionID == "" {
		return fmt.Errorf("no active session")
	}
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(absPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/sessions/"+c.sessionID+"/attachments", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeAPIError(resp.Body)
	}
	fmt.Printf("attached %s\n", filepath.Base(absPath))
	return nil
}

func (c *client) stopRun() error {
	c.mu.Lock()
	runID := c.runID
	c.mu.Unlock()
	if runID == "" {
		return fmt.Errorf("no active run")
	}
	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/runs/"+runID+"/cancel", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	fmt.Println("stop requested")
	return nil
}

func (c *client) openSocket(sessionID string) {
	c.closeSocket()
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(serverURL, "http", "ws", 1)+"/api/v1/ws", nil)
	if err != nil {
		fmt.Printf("socket error: %v\n", err)
		return
	}
	c.mu.Lock()
	c.ws = conn
	c.mu.Unlock()
	if err := conn.WriteJSON(map[string]any{
		"type": "subscribe",
		"data": map[string]any{
			"session_id": sessionID,
			"after_seq":  0,
		},
	}); err != nil {
		fmt.Printf("socket error: %v\n", err)
		return
	}
	go c.readSocket(conn)
}

func (c *client) closeSocket() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
}

func (c *client) readSocket(conn *websocket.Conn) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var message struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &message); err != nil {
			continue
		}
		c.handleSocketMessage(message.Type, message.Data)
	}
}

func (c *client) handleSocketMessage(kind string, raw json.RawMessage) {
	switch kind {
	case "session.snapshot":
		var snapshot sessionSnapshot
		if err := json.Unmarshal(raw, &snapshot); err == nil {
			c.mu.Lock()
			c.snapshot = &snapshot
			c.streamingEntryID = ""
			c.mu.Unlock()
		}
	case "session.updated":
		var data struct {
			Session    *session    `json:"session"`
			Attachment *attachment `json:"attachment"`
		}
		if err := json.Unmarshal(raw, &data); err == nil {
			c.mu.Lock()
			if c.snapshot != nil && data.Session != nil {
				c.snapshot.Session = *data.Session
				if data.Session.Status != "running" && data.Session.Status != "queued" {
					c.runID = ""
				}
			}
			if c.snapshot != nil && data.Attachment != nil {
				c.snapshot.Attachments = appendOrReplaceAttachment(c.snapshot.Attachments, *data.Attachment)
			}
			c.mu.Unlock()
			if data.Session != nil {
				fmt.Printf("\n[%s | %s]\n", data.Session.Mode, data.Session.Status)
			}
		}
	case "entry.created":
		var data struct {
			Entry entry `json:"entry"`
		}
		if err := json.Unmarshal(raw, &data); err == nil {
			c.mu.Lock()
			if c.snapshot != nil {
				c.snapshot.Entries = appendOrReplaceEntry(c.snapshot.Entries, data.Entry)
			}
			if data.Entry.Kind == "assistant_message" && data.Entry.Status == "streaming" {
				c.streamingEntryID = data.Entry.ID
				fmt.Print("\nassistant: ")
			} else {
				c.printEntry(data.Entry)
			}
			c.mu.Unlock()
		}
	case "entry.delta":
		var data struct {
			EntryID string `json:"entry_id"`
			Text    string `json:"text"`
			Entry   entry  `json:"entry"`
		}
		if err := json.Unmarshal(raw, &data); err == nil {
			c.mu.Lock()
			if c.snapshot != nil {
				c.snapshot.Entries = appendOrReplaceEntry(c.snapshot.Entries, data.Entry)
			}
			if c.streamingEntryID != data.Entry.ID && data.Entry.ID != "" {
				c.streamingEntryID = data.Entry.ID
				fmt.Print("\nassistant: ")
			}
			fmt.Print(data.Text)
			c.mu.Unlock()
		}
	case "entry.updated":
		var data struct {
			Entry entry `json:"entry"`
		}
		if err := json.Unmarshal(raw, &data); err == nil {
			c.mu.Lock()
			if c.snapshot != nil {
				c.snapshot.Entries = appendOrReplaceEntry(c.snapshot.Entries, data.Entry)
			}
			if c.streamingEntryID == data.Entry.ID && data.Entry.Status != "streaming" {
				fmt.Println()
				c.streamingEntryID = ""
			}
			c.mu.Unlock()
		}
	case "approval.updated":
		var data struct {
			Approval approval `json:"approval"`
		}
		if err := json.Unmarshal(raw, &data); err == nil {
			c.mu.Lock()
			if c.snapshot != nil {
				c.snapshot.Approvals = appendOrReplaceApproval(c.snapshot.Approvals, data.Approval)
			}
			c.mu.Unlock()
			fmt.Printf("\napproval %s: %s (%s)\n", data.Approval.ID, data.Approval.ToolName, data.Approval.Status)
		}
	}
}

func (c *client) printSnapshot(snapshot sessionSnapshot) {
	fmt.Printf("\nactive session: %s | %s | %s\n", snapshot.Session.ID, snapshot.Session.Mode, snapshot.Session.Title)
	if len(snapshot.Entries) == 0 {
		fmt.Println("no history yet")
		return
	}
	for _, item := range snapshot.Entries {
		c.printEntry(item)
	}
}

func (c *client) printEntry(item entry) {
	switch item.Kind {
	case "user_message":
		fmt.Printf("user: %s\n", item.Content)
	case "assistant_message":
		if item.Content != "" {
			fmt.Printf("assistant: %s\n", item.Content)
		}
	case "plan":
		fmt.Printf("plan:\n%s\n", item.Content)
	case "tool_call":
		fmt.Printf("tool: %s\n", item.Content)
	case "tool_result":
		fmt.Printf("result:\n%s\n", item.Content)
	case "change":
		fmt.Printf("change: %s\n", item.Content)
	case "verification":
		fmt.Printf("verify: %s\n", item.Content)
	case "approval":
		fmt.Printf("approval: %s\n", item.Content)
	case "system":
		fmt.Printf("system: %s\n", item.Content)
	}
}

func appendOrReplaceEntry(entries []entry, next entry) []entry {
	for i := range entries {
		if entries[i].ID == next.ID {
			entries[i] = next
			return entries
		}
	}
	return append(entries, next)
}

func appendOrReplaceApproval(items []approval, next approval) []approval {
	for i := range items {
		if items[i].ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func appendOrReplaceAttachment(items []attachment, next attachment) []attachment {
	for i := range items {
		if items[i].ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func decodeAPIError(body io.Reader) error {
	var out apiError
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return err
	}
	if out.Error.Message != "" {
		return errors.New(out.Error.Message)
	}
	return fmt.Errorf("request failed")
}
