Reproducing the Product
=======================

This section describes how to reproduce the full Agentic Ricing workflow on a Linux system.

Step 1: Start the backend
-------------------------

.. code-block:: bash

   cd ricing/server
   go run ./cmd/ricingd

Verify:

.. code-block:: bash

   curl http://127.0.0.1:1777/api/v1/health

Step 2: Start the browser UI
----------------------------

.. code-block:: bash

   go run ./cmd/web

Open:

.. code-block:: text

   http://localhost:5173

Step 3: Configure a provider
----------------------------

Choose one of:

- ChatGPT OAuth
- OpenAI API
- Anthropic
- Gemini
- OpenRouter
- Ollama
- LM Studio

Step 4: Create a ricing session
-------------------------------

Start a session and describe the customization goal.

Example prompt:

.. code-block:: text

   I want a dark minimal Linux rice. Inspect what is needed, identify required packages, and show me the config and theme changes before applying them.

Step 5: Use planning mode
-------------------------

If using the CLI:

.. code-block:: text

   /new
   /mode plan

Then ask for a plan first.

This verifies that the system can reason about the ricing request without mutating the machine.

Step 6: Move to build mode
--------------------------

.. code-block:: text

   /mode build

Then request application of the plan.

This verifies that the system can transition from planning to real execution.

Step 7: Test package installation path
--------------------------------------

Use a request that requires missing desktop components.

Example:

.. code-block:: text

   Install any packages needed for this rice and then patch the relevant config files.

This tests the ``install_package`` and file-edit workflow.

Step 8: Test config patching
----------------------------

Use a request that requires file modification.

Example:

.. code-block:: text

   Apply the proposed visual changes to the existing configuration.

This tests ``apply_patch``.

Step 9: Test approval flow
--------------------------

Approve or reject pending system changes when prompted.

This verifies that mutating actions are not silently executed.

Step 10: Test persistence
-------------------------

Restart the daemon and confirm that:

- sessions still exist
- attached files remain associated with the session
- previous workflow history is preserved
