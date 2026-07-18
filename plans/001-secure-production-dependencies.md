# Plan 001: Remove known vulnerabilities from the production server dependency set

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report instead of improvising. When done, update the status row for this
> plan in `plans/README.md`, unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat ea6c3f9..HEAD -- server/pyproject.toml server/uv.lock server/app/main.py .github/workflows/ci.yaml`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live files before proceeding. A
> mismatch is a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: MED
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `ea6c3f9`, 2026-07-18

## Why this matters

The public FastAPI server currently installs a lockfile containing known
vulnerabilities. The directly relevant findings include denial-of-service
issues in the form parsers exercised by `/login` and the browser job form.
The project also installs test-only packages in the production dependency
set, increasing image size and vulnerability surface. This plan upgrades the
compatible web stack, separates development dependencies, and makes a clean
dependency audit a required CI gate.

This plan does not redesign authentication, add rate limiting, or change API
behavior. Those require separate plans.

## Current state

- `server/pyproject.toml` declares all packages as production dependencies,
  including `httpx` and `pytest`:

  ```toml
  # server/pyproject.toml:6
  dependencies = [
      "fastapi",
      "httpx",
      "jinja2",
      "psycopg[binary]",
      "pytest",
      "python-multipart",
      "sqlalchemy",
      "uvicorn[standard]",
  ]
  ```

- `server/uv.lock` currently resolves these affected versions:

  ```text
  server/uv.lock:88   fastapi 0.111.0
  server/uv.lock:202  jinja2 3.1.4
  server/uv.lock:469  pytest 8.2.2
  server/uv.lock:493  python-multipart 0.0.9
  server/uv.lock:630  starlette 0.37.2
  server/uv.lock:687  ujson 5.12.1
  ```

- A `pip-audit` run on 2026-07-18 reported 21 advisories across five
  packages. Not every advisory is exploitable by DeadDrop, but the
  `python-multipart` and Starlette denial-of-service advisories are relevant
  because unauthenticated browser forms are parsed by FastAPI.

- Current published compatible targets observed on 2026-07-18 were FastAPI
  0.139.2, Starlette 1.3.1, `python-multipart` 0.0.32, Jinja2 3.1.6, pytest
  9.1.1, and `ujson` 5.13.0. Resolve current compatible versions at execution
  time; do not blindly hard-code transitive dependencies.

- `.github/workflows/ci.yaml` installs the default environment and runs the
  existing server tests, but does not audit dependencies:

  ```yaml
  # .github/workflows/ci.yaml:22
  - name: Install dependencies
    run: |
      cd server
      uv sync
  - name: Run tests
    run: |
      cd server
      export OWNER_TOKEN=test_token
      export WORKER_TOKEN=test_token
      export DATABASE_URL=sqlite:///:memory:
      export SECURE_COOKIES=false
      uv run pytest
  ```

- `server/Dockerfile` already installs with `uv sync --frozen --no-dev`.
  Preserve that convention; correctly moving test tools to the dev group will
  remove them from the production image without changing the Dockerfile.

- Starlette 1.3.1 changed `Jinja2Templates.TemplateResponse` to require
  `request` as the first positional argument. DeadDrop has eight calls using
  the removed legacy order, beginning at `server/app/main.py:73`. An initial
  isolated execution on 2026-07-18 confirmed that dependency resolution
  succeeds but four browser tests fail with `TypeError: unhashable type:
  'dict'` until these calls are migrated.

- Repository conventions to preserve:
  - Dependency resolution uses `uv` and the committed `server/uv.lock`.
  - CI commands run from `server/`.
  - Commit subjects use Conventional Commits, for example
    `fix: harden worker job lifecycle`.
  - Do not add source-code comments.
  - Do not add new tests in this plan. The existing form and API tests are the
    verification surface for this scoped dependency update.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Check lock | `cd server && UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv lock --check` | exit 0 |
| Update lock | `cd server && UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv lock --upgrade` | exit 0 and `server/uv.lock` changes |
| Sync | `cd server && UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv sync --frozen` | exit 0 |
| Server tests | `cd server && OWNER_TOKEN=test_token WORKER_TOKEN=test_token DATABASE_URL=sqlite:///:memory: SECURE_COOKIES=false uv run pytest -q` | 11 existing tests pass or more if the baseline has legitimately changed |
| Audit | `cd server && uv run pip-audit` | exit 0 and `No known vulnerabilities found` |
| Production sync | `cd server && UV_PROJECT_ENVIRONMENT=/tmp/deaddrop-prod-venv UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv sync --frozen --no-dev` | exit 0 |

## Scope

**In scope** — the only files the executor may modify:

- `server/pyproject.toml`
- `server/uv.lock`
- `server/app/main.py`
- `.github/workflows/ci.yaml`
- `plans/README.md` for the final status update only

**Out of scope** — do not touch:

- Any file under `server/app/` except `server/app/main.py`
- Any file under `server/tests/`
- Any file under `worker/`
- `server/Dockerfile`
- Authentication, token, cookie, CSRF, rate-limiting, or request-body behavior
- Database schemas or migrations
- Product naming or README copy
- New dependencies unrelated to dependency auditing

## Git workflow

- Branch: `advisor/001-secure-production-dependencies`
- Make one logical commit after all gates pass.
- Commit message: `fix: update vulnerable server dependencies`
- Do not push or open a pull request unless the operator explicitly instructs
  it.

## Steps

### Step 1: Separate runtime and development dependencies

Edit `server/pyproject.toml` so the production `dependencies` list contains
only packages imported or required by the running application:

- `fastapi`
- `jinja2`
- `psycopg[binary]`
- `python-multipart`
- `sqlalchemy`
- `uvicorn[standard]`

Move `httpx` and `pytest` into the standard uv development dependency group
and add `pip-audit` to that group. Use `[dependency-groups]` with a `dev` list;
do not create custom tiers or additional groups.

Add minimum versions only where required to exclude the known-vulnerable
versions:

- `fastapi>=0.139.2`
- `jinja2>=3.1.6`
- `python-multipart>=0.0.32`
- `pytest>=9.1.1`

Do not add a direct Starlette or `ujson` dependency. They are transitive and
must be selected through a compatible FastAPI resolution.

**Verify**:

```bash
cd server
uv lock --check
```

Expected at this intermediate step: a non-zero result saying the lockfile
needs updating. If it still exits zero, confirm the manifest was actually
changed before continuing.

### Step 2: Regenerate and inspect the lockfile

Run:

```bash
cd server
UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv lock --upgrade
UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv lock --check
```

Expected: both commands exit zero after the update. Inspect the new lockfile
and confirm:

- FastAPI is at least 0.139.2.
- Jinja2 is at least 3.1.6.
- `python-multipart` is at least 0.0.32.
- Starlette is at least 1.3.1.
- pytest is at least 9.1.1.
- `ujson`, if still present, is at least 5.13.0.
- `httpx`, pytest, and `pip-audit` belong to the dev dependency resolution and
  are not direct production dependencies of the `server` package.

Use a short read-only script or `rg` to inspect versions. Do not hand-edit
`server/uv.lock`.

**Verify**:

```bash
cd server
uv tree
```

Expected: exit zero with no resolution conflict and the minimum versions above
met.

### Step 3: Migrate the required template response call signature

Update all eight `templates.TemplateResponse` calls in
`server/app/main.py` to the Starlette 1.3.1 signature:

```python
templates.TemplateResponse(request, "template.html", context)
```

Preserve each existing template name, context dictionary, status code, and
return behavior. For the invalid-login response, keep `status_code=401` as a
keyword argument. Do not refactor route functions, change context contents,
or address unrelated FastAPI deprecations. Add no comments.

**Verify**:

```bash
rg -n 'TemplateResponse\("' server/app/main.py
```

Expected: no matches, proving no call still begins with the legacy template
name positional argument.

### Step 4: Verify application compatibility

Create or refresh the local environment from the updated lockfile, then run
the existing tests:

```bash
cd server
UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv sync --frozen
OWNER_TOKEN=test_token WORKER_TOKEN=test_token DATABASE_URL=sqlite:///:memory: SECURE_COOKIES=false uv run pytest -q
```

Expected: dependency sync exits zero and all 11 existing tests pass. Do not
change application or test code to silence deprecation warnings in this plan.
If tests still fail after the specified call-signature migration, stop and
report the exact failure. Do not make any additional framework migration.

### Step 5: Audit both the resolved environment and production shape

Run the audit from the development environment:

```bash
cd server
uv run pip-audit
```

Expected: exit zero and `No known vulnerabilities found`.

Then prove the production dependency shape installs without development tools:

```bash
cd server
UV_PROJECT_ENVIRONMENT=/tmp/deaddrop-prod-venv UV_CACHE_DIR=/tmp/deaddrop-uv-cache uv sync --frozen --no-dev
/tmp/deaddrop-prod-venv/bin/python -c "import fastapi, jinja2, multipart, psycopg, sqlalchemy, uvicorn"
```

Expected: both commands exit zero. Do not require `httpx`, pytest, or
`pip-audit` to import in the production environment.

### Step 6: Make dependency auditing a CI gate

In `.github/workflows/ci.yaml`, keep the existing server install and test
steps. Make the install reproducible with `uv sync --frozen`, then add a server
job step after the tests:

```yaml
- name: Audit dependencies
  run: |
    cd server
    uv run pip-audit
```

Do not add a new workflow, action, service, or configuration file. Keep the
audit in the existing `server-tests` job so it uses the exact environment that
was tested.

**Verify**:

```bash
cd server
OWNER_TOKEN=test_token WORKER_TOKEN=test_token DATABASE_URL=sqlite:///:memory: SECURE_COOKIES=false uv run pytest -q
uv run pip-audit
```

Expected: tests pass and the audit exits zero with no known vulnerabilities.

## Test plan

Do not write new tests in this plan. Run the existing `server/tests/test_api.py`
suite because it already exercises login form parsing, browser form handling,
authentication, job creation, receipts, log pagination, cancellation, and
worker completion. The dependency change is complete only when the entire
existing suite passes against the upgraded resolved environment.

Verification:

```bash
cd server
OWNER_TOKEN=test_token WORKER_TOKEN=test_token DATABASE_URL=sqlite:///:memory: SECURE_COOKIES=false uv run pytest -q
```

Expected: all existing tests pass.

## Done criteria

- [ ] `server/pyproject.toml` has separate runtime and `dev` dependency lists.
- [ ] The vulnerable lower versions named in Step 1 can no longer resolve.
- [ ] `server/uv.lock` was generated by uv rather than manually edited.
- [ ] `cd server && uv lock --check` exits zero.
- [ ] All existing server tests pass.
- [ ] `cd server && uv run pip-audit` exits zero with no known vulnerabilities.
- [ ] A frozen `--no-dev` production sync succeeds in a disposable environment.
- [ ] CI uses `uv sync --frozen` and runs `uv run pip-audit`.
- [ ] All eight `TemplateResponse` calls use request-first argument order.
- [ ] No files outside the in-scope list are modified.
- [ ] No source-code comments or new tests were added.
- [ ] The plan status in `plans/README.md` is updated.

## STOP conditions

Stop and report instead of improvising if:

- Any in-scope file drifted from commit `ea6c3f9` and no longer matches the
  current-state excerpts.
- A patched Starlette version cannot be resolved through a compatible current
  FastAPI release.
- Clearing the applicable advisories requires directly pinning incompatible
  transitive dependencies.
- Application changes beyond the specified request-first `TemplateResponse`
  migration or any test changes appear necessary.
- `pip-audit` reports an advisory without a compatible fixed version. Do not
  suppress, ignore, or allowlist it inside this plan.
- The existing server tests fail twice after confirming the environment was
  generated from the updated lockfile.
- Completing the work requires modifying a file outside the in-scope list.

## Maintenance notes

- Review future dependency updates with both the full server test suite and
  `pip-audit`; a valid lockfile only proves reproducibility, not security.
- Keep test and audit tooling out of the production dependency list.
- Framework deprecation cleanup is intentionally deferred. Do not mix it into
  security-only dependency updates unless a future upgrade actually removes
  the old API.
- Request-size enforcement and login abuse controls remain separate security
  work. Patched parsing removes known vulnerabilities but does not establish a
  complete resource-exhaustion policy.
