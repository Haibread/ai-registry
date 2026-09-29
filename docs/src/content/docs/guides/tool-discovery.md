---
title: Fetch an MCP server's tools
description: List a remote MCP server's tools from the MCP forms.
sidebar:
  order: 3
---

On the MCP forms, "Fetch from server" lists the tools of the remote server at
the version's Remote URL and merges the ones you tick into the tool list,
which stays editable by hand. The server guesses the endpoint (`/mcp`, `/sse`)
and connects anonymously; it refuses internal addresses unless
`TOOL_DISCOVERY_ALLOWED_CIDRS` allows them. The `tool_discovery` block of
[deploy/config.example.yaml](https://github.com/Haibread/ai-registry/blob/main/deploy/config.example.yaml) holds the settings.
