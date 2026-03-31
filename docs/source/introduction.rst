Introduction
============

Agentic Ricing is a local-first Linux ricing assistant that uses an agent runtime
to understand the user's current setup, propose visual and configuration changes,
and apply them safely on the real system.

The project is built for Linux desktop customization workflows such as:

- changing themes and color modes
- editing configuration files
- installing missing packages required for a rice
- running shell commands needed to configure the environment
- previewing and planning changes before applying them
- managing customization as a structured, session-based workflow

Unlike a static dotfiles installer, Agentic Ricing is interactive.
It can reason about a user's request, decide which tools are needed,
ask follow-up questions when necessary, and gate risky changes behind approvals.

Project Goal
------------

The goal of Agentic Ricing is to make Linux desktop ricing easier, safer,
and more reproducible by combining AI planning with real local system actions.

Why this matters
----------------

Traditional ricing usually requires:

- knowing which packages to install
- manually editing multiple config files
- understanding how a WM or DE is laid out
- trial-and-error with theme changes
- manually keeping track of what changed

Agentic Ricing turns that into a guided workflow with preview, execution,
and persistence.
