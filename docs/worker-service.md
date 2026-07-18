# Worker service

The worker should run continuously under a non-root operating-system account. A service manager restarts it after reboots or unexpected exits.

## Linux systemd user service

Install the binary:

```bash
install -Dm755 deaddrop-worker "$HOME/.local/bin/deaddrop-worker"
mkdir -p "$HOME/.config/deaddrop"
```

Create the manifest:

```bash
deaddrop-worker init \
  --repo /absolute/path/to/project \
  --output "$HOME/.config/deaddrop/worker.json" \
  --verify "go test ./..."
```

Create `$HOME/.config/deaddrop/worker.env` with mode `0600`:

```text
DEADDROP_SERVER=https://deaddrop.example.com
WORKER_TOKEN=replace-with-worker-token
```

Create `$HOME/.config/systemd/user/deaddrop-worker.service`:

```ini
[Unit]
Description=DeadDrop worker
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%h/.config/deaddrop/worker.env
ExecStart=%h/.local/bin/deaddrop-worker run --server ${DEADDROP_SERVER} --token ${WORKER_TOKEN} --manifest %h/.config/deaddrop/worker.json --agent gemini
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

Enable it:

```bash
chmod 600 "$HOME/.config/deaddrop/worker.env" "$HOME/.config/deaddrop/worker.json"
systemctl --user daemon-reload
systemctl --user enable --now deaddrop-worker
systemctl --user status deaddrop-worker
```

To keep user services running after logout:

```bash
loginctl enable-linger "$USER"
```

Logs:

```bash
journalctl --user -u deaddrop-worker -f
```

## macOS launch agent

Install the binary and manifest under your user account. Store the token in a user-readable-only environment file or secret manager, then use a LaunchAgent to invoke:

```text
$HOME/.local/bin/deaddrop-worker run --server https://deaddrop.example.com --token TOKEN --manifest $HOME/.config/deaddrop/worker.json --agent gemini
```

The process must not run as root. Configure `KeepAlive` and `RunAtLoad` in the LaunchAgent.

## Windows

Run the released `.exe` as the signed-in user from Task Scheduler or a user-level service wrapper. Configure startup at login, restart on failure, and store the worker token with user-only permissions.

## Updating

Stop the service, verify the new binary checksum, replace the binary atomically, confirm `deaddrop-worker version`, and restart the service. The pending-result queue remains under the user configuration directory and survives binary replacement.
