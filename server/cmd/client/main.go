package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/wowtuff/ricing/agent"
	"github.com/wowtuff/ricing/tools/toolset"
)

const version = "0.1.0"

const serverURL = "http://127.0.0.1:1777"

func main() {
	fmt.Printf("ricing %s — ricing agent\n\n", version)

	if !checkServer() {
		fmt.Println("error: server not running. start with 'go run cmd/web/main.go'")
		return
	}

	connected := checkConnection()
	if connected {
		fmt.Println("ready (using chatgpt via OAuth)\n")
	} else {
		fmt.Println("not linked. run /connect to link your account\n")
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch input {
		case "/quit", "/exit", "/q":
			fmt.Println("bye!")
			return

		case "/help", "/h", "/?":
			fmt.Println("commands:")
			fmt.Println("  /connect   link your chatgpt account")
			fmt.Println("  /status    check status")
			fmt.Println("  /help      show this")
			fmt.Println("  /quit      exit")
			fmt.Println()
			fmt.Println("type your message to start")

		case "/connect":
			doConnect()

		case "/status":
			printStatus()

		default:
			if !checkConnection() {
				fmt.Println("not linked. run /connect first")
				continue
			}
			runPrompt(input)
		}
	}
}

func checkServer() bool {
	resp, err := http.Get(serverURL + "/api/v1/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func checkConnection() bool {
	resp, err := http.Get(serverURL + "/api/v1/providers")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var data struct {
		Providers []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false
	}
	for _, p := range data.Providers {
		if p.State == "connected" {
			return true
		}
	}
	return false
}

func printStatus() {
	if checkConnection() {
		fmt.Println("status: connected (chatgpt OAuth)")
	} else {
		fmt.Println("status: not linked")
	}
}

func doConnect() {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│  linking chatgpt...                 │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("→ open the web UI to connect:")
	fmt.Println("  http://localhost:5173")
	fmt.Println()
	fmt.Println("in the web UI:")
	fmt.Println("  1. click the status badge (top right)")
	fmt.Println("  2. select 'chatgpt (oauth)'")
	fmt.Println("  3. click 'connect with chatgpt'")
	fmt.Println()
	fmt.Println("after connecting, your cli will automatically use chatgpt")
	fmt.Println("type /status to check connection status")
}

func runPrompt(prompt string) {
	fmt.Println()

	reg := toolset.NewDefaultRegistry()
	err := agent.RunStream(context.Background(), reg, agent.RunOptions{
		Backend: "prov_openai_oauth_codex",
		Model:   "gpt-5.2-codex",
	}, prompt, agent.StreamSink{
		OnDelta: func(text string) {
			fmt.Print(text)
		},
		OnToolCall: func(call agent.StreamToolCall) {
			fmt.Printf("\n[tool: %s]\n", call.Name)
		},
		OnToolResult: func(res agent.ToolResult) {
			fmt.Printf("[result: %s]\n", res.Output)
		},
	})
	if err != nil {
		fmt.Printf("\nerror: %v\n", err)
	}
	fmt.Println()
}
