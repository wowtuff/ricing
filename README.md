# Agentic Ricing

## Documentation

Full documentation is available here:
[https://wowtuff.github.io/ricing/](https://wowtuff.github.io/ricing/)

Agentic Ricing is an AI-powered Linux ricing assistant that can inspect your setup, plan changes, install required packages, patch configuration files, switch appearance modes, and guide the user through applying a rice safely through a browser UI and CLI.

## What it does

Agentic Ricing helps automate Linux desktop customization by combining:

- system inspection
- package installation
- config file reading and patching
- dark/light mode switching
- service log inspection
- interactive planning and approvals

It is designed to work as a local agent runtime for applying ricing changes on a real Linux system.

## Reproducing the project

### Prerequisites

- Go 1.25+
- Git
- Linux machine
- A supported model provider:
  - ChatGPT OAuth
  - OpenAI API key
  - Anthropic API key
  - Gemini API key
  - OpenRouter API key
  - Ollama
  - LM Studio

### Clone the repository

```bash
git clone https://github.com/wowtuff/ricing.git
cd ricing/server
```

### Install dependencies

```bash
go mod tidy
```

## to start backend and ui together
```bash
go run cmd/ricingd/main.go --ui-dir cmd/server/ui
```

### Start the backend

```bash
go run ./cmd/ricingd
```

The daemon runs by default at:

```text
http://127.0.0.1:1777
```


### Start the web UI

In another terminal:

```bash
go run ./cmd/web
```

Open:

```text
http://localhost:5173
```

### Optional: start the CLI

```bash
go run ./cmd/client
```

## Basic workflow

1. Start the daemon
2. Open the browser UI
3. Configure a model provider
4. Create a session
5. Ask Agentic Ricing to inspect and plan a rice
6. Review the plan and approve changes
7. Let it install packages, patch configs, and apply appearance changes

## Tools used by the agent

The current tool layer includes:

- `notify`
- `cmd`
- `install_package`
- `read_file`
- `apply_patch`
- `get_system_info`
- `get_service_logs`
- `set_color_mode`
- `update_plan`
- `request_user_input`

These tools let Agentic Ricing inspect the current system, understand the Linux desktop environment, install dependencies, edit configuration files, switch visual mode, and interact with the user safely.
