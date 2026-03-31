Available Tools
===============

Agentic Ricing uses a tool registry to perform real Linux customization work.
Excluding the internal demo math tool, the active toolset is composed of the
following runtime tools.

Tool List
---------

``notify``
   Sends a desktop notification when the workflow finishes.

``cmd``
   Executes shell commands with safety checks. It supports inspection and
   controlled command execution while blocking dangerous system operations.

``install_package``
   Installs required packages using the correct package manager for the current
   Linux distribution.

``read_file``
   Reads configuration files safely, either completely or by line range.

``apply_patch``
   Applies safe line-based edits to files. This is the main mechanism for
   modifying user configuration during a rice.

``get_system_info``
   Collects Linux system, desktop session, package manager, toolkit, and
   appearance information.

``get_service_logs``
   Reads ``journalctl`` logs for a specific service or systemd unit. Useful for
   debugging desktop components involved in the rice.

``set_color_mode``
   Switches Linux desktop and toolkit settings between dark and light mode,
   including GTK, Cinnamon, Xfce, Plasma, and Qt-related appearance backends.

``update_plan``
   Records a structured plan for the current task so the user can review the
   intended steps before execution.

``request_user_input``
   Asks the user a concise multiple-choice question when clarification is needed.

How these tools support ricing
------------------------------

Together, these tools allow Agentic Ricing to:

- detect the user's Linux environment
- inspect the current appearance state
- understand which package managers and themes are available
- read current config files
- plan a customization workflow
- ask the user for style or mode decisions
- install missing dependencies
- patch configuration files safely
- switch toolkit and desktop color mode
- debug affected services if needed
- notify the user when the workflow is complete
