package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wowtuff/ricing/agent"
	"github.com/wowtuff/ricing/tools/toolset"
)

func main() {
	fmt.Println("Ricing Agent")
	fmt.Println()

	cfg, err := loadSavedConfig()
	if err == nil && cfg != nil {
		fmt.Printf("Using saved config: %s / %s\n", cfg.Backend, cfg.Model)
		useSaved := promptYesNo("Use saved config? (Y/n)", true)
		if useSaved {
			runWithConfig(cfg)
			return
		}
	}

	choice := promptChoice([]string{
		"ChatGPT (OAuth)",
		"OpenAI API",
		"Anthropic API",
		"Gemini API",
		"OpenRouter API",
		"Mistral API",
		"Local (Ollama/LM Studio)",
	})

	var newCfg agent.Config
	switch choice {
	case 0:
		newCfg.Backend = "chatgpt"
		fmt.Println("\nStarting OAuth login...")
		_, token, err := agent.ConnectOpenAI(context.Background(), agent.ConnectOptions{OpenBrowser: true})
		if err != nil {
			log.Fatal("OAuth failed: ", err)
		}
		fmt.Println("Login successful!")
		model := promptString("Model (default: gpt-5.2-codex)", "gpt-5.2-codex")
		newCfg.Model = model
		_ = token

	case 1:
		newCfg.Backend = "openai"
		newCfg.APIKey = promptString("OpenAI API Key", "")
		newCfg.Model = promptString("Model (default: gpt-4)", "gpt-4")

	case 2:
		newCfg.Backend = "anthropic"
		newCfg.APIKey = promptString("Anthropic API Key", "")
		newCfg.Model = promptString("Model (default: claude-sonnet-4-20250514)", "claude-sonnet-4-20250514")

	case 3:
		newCfg.Backend = "gemini"
		newCfg.APIKey = promptString("Gemini API Key", "")
		newCfg.Model = promptString("Model (default: gemini-2.0-flash)", "gemini-2.0-flash")

	case 4:
		newCfg.Backend = "openrouter"
		newCfg.APIKey = promptString("OpenRouter API Key", "")
		newCfg.Model = promptString("Model (default: openai/gpt-4o)", "openai/gpt-4o")

	case 5:
		newCfg.Backend = "mistral"
		newCfg.APIKey = promptString("Mistral API Key", "")
		newCfg.Model = promptString("Model (default: mistral-large-latest)", "mistral-large-latest")

	case 6:
		newCfg.Backend = "local"

		// Ask which local model runner they want to use
		fmt.Println("\nLocal Model Setup")
		localChoice := promptChoice([]string{
			"Ollama (https://ollama.com)",
			"LM Studio (https://lmstudio.ai)",
			"llama.cpp server",
			"Custom URL",
		})

		switch localChoice {
		case 0: // Ollama
			if !isOllamaInstalled() {
				fmt.Println("\nOllama is not installed.")
				install := promptYesNo("Install Ollama now?", false)
				if install {
					openBrowser("https://ollama.com")
				}
				fmt.Println("\nAfter installing, run this again!")
				os.Exit(0)
			}
			if !isServiceRunning("http://localhost:11434") {
				fmt.Println("\nOllama is not running.")
				start := promptYesNo("Start Ollama now?", true)
				if start {
					if err := startOllama(); err != nil {
						fmt.Println("Failed to start:", err)
						os.Exit(1)
					}
				} else {
					fmt.Println("Please start Ollama and run again.")
					os.Exit(0)
				}
			}
			newCfg.URL = "http://localhost:11434/v1"

		case 1: // LM Studio
			if !isServiceRunning("http://localhost:1234") {
				fmt.Println("\nLM Studio is not running.")
				fmt.Println("Please start LM Studio and load a model, then press Enter...")
				fmt.Println("(In LM Studio: Click a model -> Load -> Wait for 'Local Server' to appear)")
				promptString("Press Enter when ready", "")
			}
			newCfg.URL = "http://localhost:1234/v1"

		case 2: // llama.cpp server
			if !isServiceRunning("http://localhost:8080") {
				fmt.Println("\nllama.cpp server is not running.")
				fmt.Println("Start it with: ./server -m model.gguf --host 0.0.0.0 -c 4096")
				fmt.Println("\nOr download a model and run:")
				fmt.Println("  https://github.com/ggerganov/llama.cpp/tree/master/examples/server")
				os.Exit(0)
			}
			newCfg.URL = "http://localhost:8080/v1"

		case 3: // Custom URL
			newCfg.URL = promptString("Server URL", "http://localhost:11434/v1")
		}

		// Try to get list of available models
		if models := getLocalModels(newCfg.URL); len(models) > 0 {
			fmt.Println("\nAvailable models:")
			for i, m := range models {
				fmt.Printf("  %d. %s\n", i+1, m)
			}
			fmt.Println("  0. Enter custom name")
			modelChoice := promptChoice(models)
			if modelChoice < len(models) {
				newCfg.Model = models[modelChoice]
			} else {
				newCfg.Model = promptString("Model name", "")
			}
		} else {
			newCfg.Model = promptString("Model name (e.g., llama3, qwen2.5)", "")
		}
	}

	save := promptYesNo("Save config for next time? (Y/n)", true)
	if save {
		if err := saveConfig(&newCfg); err != nil {
			fmt.Println("Warning: Failed to save config:", err)
		} else {
			fmt.Println("Config saved!")
		}
	}

	runWithConfig(&newCfg)
}

func runWithConfig(cfg *agent.Config) {
	fmt.Println("\nRunning agent...")
	prompt := promptString("Prompt (or press Enter for default)", "What is 6 times 7?")

	reg := toolset.NewDefaultRegistry()
	answer, err := agent.Run(context.Background(), reg, prompt, agent.WithConfig(*cfg))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nResponse:")
	fmt.Println(answer)
}

func loadSavedConfig() (*agent.Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg agent.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *agent.Config) error {
	path := configPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ricing", "config.json")
}

func promptChoice(options []string) int {
	fmt.Println("Choose a provider:")
	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}
	fmt.Print("\n> ")

	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		var n int
		fmt.Sscanf(line, "%d", &n)
		if n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Printf("Invalid choice (1-%d): ", len(options))
	}
}

func promptString(label, def string) string {
	fmt.Printf("%s", label)
	if def != "" {
		fmt.Printf(" [%s]", def)
	}
	fmt.Print(": ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptYesNo(label string, def bool) bool {
	yesNo := "Y/n"
	if !def {
		yesNo = "y/N"
	}
	fmt.Printf("%s [%s]: ", label, yesNo)

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return def
	}
	return strings.ToLower(line) == "y" || strings.ToLower(line) == "yes"
}

func isOllamaInstalled() bool {
	// Try to run ollama --version
	cmd := exec.Command("ollama", "--version")
	return cmd.Run() == nil
}

func isOllamaRunning() bool {
	return isServiceRunning("http://localhost:11434")
}

func isServiceRunning(url string) bool {
	resp, err := http.Get(url + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func getLocalModels(baseURL string) []string {
	resp, err := http.Get(baseURL + "/api/tags")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}
	return models
}

func startOllama() error {
	cmd := exec.Command("ollama", "serve")
	cmd.Start()
	return cmd.Err
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
