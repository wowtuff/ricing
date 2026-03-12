package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const version = "0.1.0"

func main() {
	port := flag.String("port", "5173", "port for web UI")
	flag.Parse()

	fmt.Printf("ricing %s — ricing agent\n\n", version)

	checkServer()

	startWebUI(*port)
}

func checkServer() {
	resp, err := http.Get("http://127.0.0.1:1777/api/v1/health")
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("starting ricingd server...")
		startServer()
		time.Sleep(1 * time.Second)
	}
	resp, _ = http.Get("http://127.0.0.1:1777/api/v1/health")
	if resp != nil {
		resp.Body.Close()
	}
	fmt.Println("server running at http://127.0.0.1:1777")
}

func startServer() {
	cmd := exec.Command("go", "run", "cmd/ricingd/main.go")
	cmd.Dir = getServerDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
}

func startWebUI(port string) {
	webDir := filepath.Join(getWebDir(), "web")
	fs := http.FileServer(http.Dir(webDir))
	http.Handle("/", fs)

	url := fmt.Sprintf("http://localhost:%s", port)
	fmt.Printf("web UI: %s\n", url)
	openBrowser(url)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getServerDir() string {
	wd, _ := os.Getwd()
	return wd
}

func getWebDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filename)
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
	cmd.Start()
}
