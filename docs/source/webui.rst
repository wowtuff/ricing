Web Interface
=============

Overview
--------

The browser interface is the visual control surface for Agentic Ricing.

It is designed to let users:

- choose a model provider
- configure credentials or local endpoints
- start a ricing session
- view streaming responses
- inspect proposed actions
- preview and understand system changes before applying them

Ricing-focused role of the UI
-----------------------------

The web interface is important because Agentic Ricing is not just a terminal tool.
It is also a visual ricing workflow product.

In the intended product flow, the browser UI is where the user sees:

- the current desktop or WM/DE context
- the changes the system intends to apply
- tool activity
- confirmation points before actual system mutation

This makes the ricing process more understandable and safer than editing configs blindly.

Provider setup
--------------

The UI supports selecting from:

- ChatGPT via OAuth
- OpenAI API
- Anthropic API
- Gemini
- OpenRouter
- Ollama
- LM Studio

The default browser workflow is ideal for users who want a guided ricing experience
without driving everything manually from the shell.
