package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const serverURL = "http://127.0.0.1:1777"

func main() {
	fmt.Println("ricing launcher")
	fmt.Println()
	if !serverRunning() {
		if err := startServer(); err != nil {
			fmt.Println("could not start backend:", err)
			return
		}
		if err := waitForServer(); err != nil {
			fmt.Println("backend did not come up:", err)
			return
		}
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("1. localhost ui")
	fmt.Println("2. cli")
	fmt.Print("> ")
	line, _ := reader.ReadString('\n')
	switch strings.TrimSpace(line) {
	case "1":
		openBrowser(serverURL)
		fmt.Println("opened", serverURL)
	case "2":
		runClient()
	default:
		fmt.Println("invalid choice")
	}
}

func serverRunning() bool {
	resp, err := http.Get(serverURL + "/api/v1/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func startServer() error {
	cmd := exec.Command("go", "run", "cmd/ricingd/main.go", "--ui-dir", "cmd/server/ui")
	cmd.Dir = repoRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func waitForServer() error {
	started := time.Now()
	for time.Since(started) < 20*time.Second {
		if serverRunning() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out")
}

func runClient() {
	cmd := exec.Command("go", "run", "cmd/client/main.go")
	cmd.Dir = repoRoot()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func repoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
