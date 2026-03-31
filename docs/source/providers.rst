Providers
=========

Supported providers
-------------------

Hosted providers
^^^^^^^^^^^^^^^^

- OpenAI OAuth (Codex path)
- OpenAI API Key
- OpenRouter
- Anthropic
- Gemini

Local providers
^^^^^^^^^^^^^^^

- Ollama
- LM Studio

Why provider flexibility matters for ricing
-------------------------------------------

Linux ricing tasks vary from lightweight planning to real patching and execution.
Supporting both cloud and local models allows Agentic Ricing to fit different user needs:

- privacy-sensitive local setups
- low-latency local experimentation
- more capable cloud reasoning for bigger transformations

Authentication models
---------------------

OAuth
^^^^^

Used for the OpenAI OAuth/Codex provider.

API key
^^^^^^^

Used for OpenAI API, Anthropic, Gemini, and OpenRouter.

Local URL
^^^^^^^^^

Used for Ollama and LM Studio.
