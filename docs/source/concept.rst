Core Concept
============

What "ricing" means here
------------------------

In Agentic Ricing, "ricing" means changing the appearance and behavior of a Linux
desktop environment or window manager by modifying the actual system state.

That can include:

- changing colors and themes
- editing config files under the user's setup
- installing packages required for a given rice
- adjusting shell, bar, or window-manager behavior
- applying patches to configuration
- running commands to enact or verify changes

Agentic, not static
-------------------

Agentic Ricing is not just a theme pack or dotfiles repository.

It is agentic because it can:

- inspect the user's request
- decide which tool sequence is needed
- build a plan first
- ask the user a question if something is ambiguous
- wait for approval before mutating the system
- apply and verify changes step by step

Modes
-----

The runtime exposes these working modes:

- ``plan``
- ``build``
- ``auto``

Plan mode focuses on analysis and planning.

Build mode allows system-changing execution with safety controls.

Auto mode balances planning and execution depending on the task.
