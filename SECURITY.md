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

### The verification step is the sharpest edge

Verification is the weakest part of the trust model, and it is easy to miss
because the commands themselves are yours.

The agent edits files, and then DeadDrop **executes those files** with your
commands. `go test ./...` or `npm test` runs code the agent just wrote, as the
worker user, with whatever credentials that user has: SSH keys, cloud tokens,
package registry credentials, a logged-in CLI session. Git worktree isolation
does nothing about this, because the danger is not what the agent changed in the
repository but what the resulting process can reach outside it.

"DeadDrop never commits" is a real guarantee about your repository. It is not a
guarantee about your machine.

Until the verification step is sandboxed, treat every job as running untrusted
code with your permissions:

- run the worker as a dedicated user with the narrowest useful credentials
- keep cloud, registry, and deployment credentials out of that user's environment
- prefer a container, VM, or dedicated machine for the worker
- consider cutting network access for the verification step

Expected behavior, not a vulnerability:

- the configured agent can read and modify files available to the worker user
- custom commands can execute arbitrary local programs
- verification commands are trusted, but the code they execute is agent-authored
- returned patches may contain malicious or incorrect code
- Git worktree isolation does not restrict network or operating-system access

Security-sensitive defects include authentication bypass, cross-site request forgery, stale-attempt result acceptance, path escape from the configured workspace scope, token disclosure, unsafe migration behavior, or execution outside the documented worker trust boundary.
