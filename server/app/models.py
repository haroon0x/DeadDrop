QUEUED = "queued"
RUNNING = "running"
COMPLETED = "completed"
FAILED = "failed"
CANCELLED = "cancelled"

STATUSES = {QUEUED, RUNNING, COMPLETED, FAILED, CANCELLED}

MOCK = "mock"
DEFAULT_AGENT = ""

AGENTS = {
    "": "Worker default",
    "claude": "Claude Code",
    "codex": "Codex CLI",
    "aider": "Aider",
    "cursor": "Cursor CLI",
    "opencode": "OpenCode",
    "gemini": "Gemini CLI",
    MOCK: "Mock (deterministic demo)",
}
