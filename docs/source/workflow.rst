Ricing Workflow
===============

Agentic Ricing follows a structured workflow for applying Linux customization.

1. User request
---------------

The workflow begins with a natural-language instruction such as:

- "Make my Hyprland setup minimal and dark"
- "Apply a Catppuccin-inspired rice"
- "Install the missing packages and patch my bar config"
- "Change monitor scale from 2 to 3"
- "Reduce rounded borders of window"
- "Change my keybindings to something more intuitive as..."

2. Planning
-----------

The system can create an internal or user-visible plan before changing anything.

This is especially useful when the request involves:

- multiple files
- package dependencies
- potentially destructive edits
- uncertain system state

3. Inspection and context building
----------------------------------

The agent builds context from:

- the current session history
- attached files
- previous prompts and responses
- system-facing tool outputs

4. Preview and clarification
----------------------------

If the request is unclear, the agent can ask the user a structured question.
This keeps the workflow interactive rather than blindly applying defaults.

5. Execution
------------

Once enough context is available, the agent can perform concrete ricing actions,
such as:

- patching configuration files
- installing missing packages
- running shell commands
- applying color or mode changes

6. Approval gate
----------------

Mutating actions can be held behind explicit approvals.
This is important for real-system customization because file edits and package
installs should not happen silently.

7. Verification
---------------

After execution, the system records outputs and can add verification notes for
commands that were run.

8. Persistence
--------------

Sessions, approvals, attachments, and history are saved to disk so the workflow
can continue across restarts.
