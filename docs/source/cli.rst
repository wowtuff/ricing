CLI Interface
=============

Overview
--------

The CLI provides a more advanced and session-oriented way to use Agentic Ricing.

It is especially useful for:

- reproducible workflows
- approval handling
- attachment-driven configuration tasks
- switching between sessions
- controlling mode explicitly

Commands
--------

``/connect``
   Connect the default provider

``/status``
   Show provider and active session state

``/sessions``
   List sessions

``/new``
   Create a new session

``/use <id>``
   Switch to a session

``/mode <auto|plan|build>``
   Set the active execution mode

``/approve <id>``
   Approve a pending action

``/reject <id>``
   Reject a pending action

``/files``
   List files attached to the active session

``/attach <path>``
   Attach a file to the current session

``/stop``
   Cancel the active run

CLI example
-----------

A typical Agentic Ricing CLI flow:

.. code-block:: text

   /new
   /mode plan

Then send a request such as:

.. code-block:: text

   Create a minimal dark rice for my Linux setup and explain which packages and config changes are needed.

After reviewing the plan:

.. code-block:: text

   /mode build

Then send:

.. code-block:: text

   Apply the planned changes.

If a mutating step requires approval, the CLI will let the user approve or reject it.
