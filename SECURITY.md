# Security policy

## Supported versions

DeadDrop is pre-1.0. Security fixes are provided on the latest release and the `main` branch. Older releases may require upgrading rather than receiving a backport.

## Reporting

Use GitHub's private vulnerability reporting or Security Advisory flow for `haroon0x/DeadDrop`. Do not include exploit details, tokens, private prompts, patches, or repository contents in a public issue.

Include the affected version, component, reproduction conditions, impact, and any proposed mitigation. Maintainers will acknowledge the report, validate scope, coordinate a fix, and publish an advisory when appropriate.

## Security model

DeadDrop separates a hosted control plane from a local execution worker, but the coding agent is not sandboxed. A task may cause the agent to run commands with the worker user's permissions.

Operators must:

- run the worker as a dedicated non-root user
- expose only intended Git workspaces
- protect owner and worker tokens
- use HTTPS outside localhost
- use persistent PostgreSQL with backups
- review returned patches before applying them
- avoid placing DeadDrop control files or unrelated credentials inside agent workspaces

Expected behavior, not a vulnerability:

- the configured agent can read and modify files available to the worker user
- custom commands can execute arbitrary local programs
- verification commands are trusted local configuration
- returned patches may contain malicious or incorrect code
- Git worktree isolation does not restrict network or operating-system access

Security-sensitive defects include authentication bypass, cross-site request forgery, stale-attempt result acceptance, path escape from the configured workspace scope, token disclosure, unsafe migration behavior, or execution outside the documented worker trust boundary.
