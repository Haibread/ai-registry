---
title: Manage registry-wide tags
description: The tag vocabulary publishers pick from.
sidebar:
  order: 4
---

Server Admins manage the tag vocabulary publishers pick from at
`/admin/tags` or `/api/v1/tags`. Tags can also be declared in configuration
(`instance_tags` key, `INSTANCE_TAGS` JSON, or the chart's `api.instanceTags`);
those are reconciled on start and read-only in the UI and API.
