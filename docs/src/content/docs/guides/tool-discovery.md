---
title: Fetch an MCP server's tools and protocol revisions
description: List a remote MCP server's tools and supported protocol revisions from the MCP forms.
sidebar:
  order: 3
---

On the MCP forms, "Fetch from server" lists the tools of the remote server at
the version's Remote URL and merges the ones you tick into the tool list,
which stays editable by hand. The server guesses the endpoint (`/mcp`, `/sse`)
and connects anonymously; it refuses internal addresses unless
`TOOL_DISCOVERY_ALLOWED_CIDRS` allows them. The `tool_discovery` block of
[deploy/config.example.yaml](https://github.com/Haibread/ai-registry/blob/main/deploy/config.example.yaml) holds the settings.

"Detect from server", next to the Protocol versions field, uses the same
connection to find the protocol revisions the server supports: each revision
the registry's MCP client speaks is offered in its own handshake, and the ones
the server accepts unchanged are shown for review. "Use these" replaces the
list; "Keep my list" leaves it as it is and offers the detected revisions as
suggestions while typing. Revisions newer than the registry's MCP client
cannot be detected and are typed in by hand.
