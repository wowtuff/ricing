API
===

Base URL
--------

.. code-block:: text

   http://127.0.0.1:1777

Main routes
-----------

Core
^^^^

- ``GET /api/v1/health``
- ``GET /api/v1/meta``
- ``GET /api/v1/tools``

Providers
^^^^^^^^^

- ``GET /api/v1/providers/types``
- ``GET /api/v1/providers``
- provider action routes under ``/api/v1/providers/...``

Runs
^^^^

- ``POST /api/v1/runs``
- run action routes under ``/api/v1/runs/...``

Sessions
^^^^^^^^

- ``GET /api/v1/sessions``
- ``POST /api/v1/sessions``
- session action routes under ``/api/v1/sessions/...``

Approvals
^^^^^^^^^

- approval routes under ``/api/v1/approvals/...``

WebSocket
^^^^^^^^^

- ``/api/v1/ws``

Why the API matters for this project
------------------------------------

The API is not just an implementation detail.
It is what makes Agentic Ricing programmable, inspectable, and extensible.

Because the web UI and CLI both talk to the same backend, the product can be:

- reproduced consistently
- integrated into other tools
- tested independently of the UI
