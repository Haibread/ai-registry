---
title: Seed the catalog from a file
description: Declare publishers and MCP servers in a file the server upserts on start.
sidebar:
  order: 2
---

Point `BOOTSTRAP_FILE` (or `--bootstrap-file`) at a YAML or JSON file and the
server upserts the publishers and MCP servers it declares on every
start. Existing rows are left untouched, except that newly declared `tools[]`
are backfilled; role grants are managed through the API, not this file. See
[deploy/bootstrap.example.yaml](https://github.com/Haibread/ai-registry/blob/main/deploy/bootstrap.example.yaml).
