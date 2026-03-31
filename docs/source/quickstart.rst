Quick Start
===========

Prerequisites
-------------

Before running Agentic Ricing, make sure you have:

- Go 1.25 or newer
- Git
- a Linux system for testing the ricing workflow
- one supported model provider:
  
  - ChatGPT OAuth
  - OpenAI API key
  - Anthropic API key
  - Gemini API key
  - OpenRouter API key
  - Ollama
  - LM Studio

Clone the repository
--------------------

.. code-block:: bash

   git clone https://github.com/wowtuff/ricing.git
   cd ricing/server

Install dependencies
--------------------

.. code-block:: bash

   go mod tidy

Start the daemon
----------------

.. code-block:: bash

   go run ./cmd/ricingd

By default the daemon runs on:

.. code-block:: text

   http://127.0.0.1:1777

Start the browser UI
--------------------

In another terminal:

.. code-block:: bash

   go run ./cmd/web

This serves the UI on:

.. code-block:: text

   http://localhost:5173

Start the CLI
-------------

Optional terminal client:

.. code-block:: bash

   go run ./cmd/client
