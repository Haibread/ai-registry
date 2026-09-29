---
title: Deploy on Kubernetes
description: Container images and the Helm chart.
sidebar:
  order: 5
---

Images are published to GHCR as `ghcr.io/haibread/ai-registry/server` and
`ghcr.io/haibread/ai-registry/web`. The Helm chart is published as an OCI
artifact:

```bash
helm install ai-registry oci://ghcr.io/haibread/ai-registry/charts/ai-registry --version <chart-version>
```

Chart values, including the optional CloudNativePG database, are documented in
[deploy/helm/ai-registry/README.md](https://github.com/Haibread/ai-registry/blob/main/deploy/helm/ai-registry/README.md).
Operating a deployment is covered by the [operations runbook](/guides/runbook/) and
[database backup and restore](/guides/db-backup/).

Releases are cut with [scripts/release.sh](https://github.com/Haibread/ai-registry/blob/main/scripts/release.sh), never by
tagging by hand — see [Releases](https://github.com/Haibread/ai-registry/blob/main/CONTRIBUTING.md#releases).
