Architecture
============

System layout
-------------

Agentic Ricing is built from several cooperating services and clients.

Daemon
------

The daemon is the core backend. It initializes:

- provider service
- session service
- run service
- tool registry
- API routes
- optional static UI serving

Session service
---------------

The session service stores:

- session metadata
- user and assistant entries
- plans
- questions
- tool calls
- tool results
- approvals
- attachments

All of this is persisted on disk.

Run service
-----------

The run service is responsible for:

- creating runs
- creating assistant streaming entries
- building the model input
- handling tool execution
- enforcing approval rules
- updating run and session status

Provider service
----------------

The provider service abstracts model backends and supports:

- OAuth-backed providers
- API-key providers
- local providers

Clients
-------

Web UI
   Browser-based interaction and preview flow

CLI
   Terminal-driven session control

Persistence
-----------

Session state is stored in:

.. code-block:: text

   ~/.ricing/sessions

OAuth token cache is stored in:

.. code-block:: text

   ~/.codex/auth.json
