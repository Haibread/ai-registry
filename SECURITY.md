# Security policy

## Reporting a vulnerability

Report it privately through
[GitHub's private vulnerability reporting](https://github.com/Haibread/ai-registry/security/advisories/new).
The report opens a draft advisory that only the maintainers and you can see,
where the fix is discussed and prepared.

Do not open a public issue, pull request or discussion for a vulnerability:
describing it in public discloses it before it is fixed.

A useful report states the affected version, the steps to reproduce and the
impact as you see it.

## Supported versions

Fixes ship in a new release; only the latest release is supported.

## Scope

In scope is what this repository builds and publishes:

- the registry server and its HTTP API, including authentication,
  authorization and the OIDC login flow;
- the web SPA and the nginx image that serves it;
- the container images and the Helm chart, including their default values.

Out of scope:

- a vulnerability in a dependency that is already public upstream: report it
  there, and the dependency bot brings the fix in;
- a finding that needs an already-compromised host or account;
- scanner output with no path to exploitation.

## What happens next

Reports are handled on a best-effort basis.

1. The report is acknowledged.
2. It is assessed for impact and reproducibility.
3. The fix is prepared in the advisory's private fork.
4. A release ships the fix.
5. The advisory is published, with a CVE requested through GitHub when the
   impact warrants one, and you are credited unless you prefer otherwise.
