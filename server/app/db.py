import os
import sqlite3
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def db_path() -> str:
    path = os.getenv("SQLITE_PATH", "./deaddrop.db")
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    return path


@contextmanager
def connect():
    conn = sqlite3.connect(db_path(), timeout=30)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    try:
        yield conn
        conn.commit()
    finally:
        conn.close()


def init_db() -> None:
    with connect() as conn:
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS workers (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              name TEXT UNIQUE NOT NULL,
              token_hash TEXT,
              last_seen_at TEXT,
              created_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS jobs (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              title TEXT NOT NULL,
              prompt TEXT NOT NULL,
              repo_alias TEXT NOT NULL DEFAULT 'default',
              worker_name TEXT NOT NULL DEFAULT 'local',
              status TEXT NOT NULL,
              created_at TEXT NOT NULL,
              updated_at TEXT NOT NULL,
              started_at TEXT,
              completed_at TEXT,
              error_message TEXT,
              final_summary TEXT,
              git_diff TEXT,
              exit_code INTEGER
            );

            CREATE TABLE IF NOT EXISTS job_logs (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
              timestamp TEXT NOT NULL,
              stream TEXT NOT NULL,
              content TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS worker_repos (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              worker_name TEXT NOT NULL,
              repo_alias TEXT NOT NULL,
              display_name TEXT NOT NULL,
              last_seen_at TEXT NOT NULL,
              created_at TEXT NOT NULL,
              UNIQUE(worker_name, repo_alias)
            );

            CREATE INDEX IF NOT EXISTS idx_jobs_status_worker ON jobs(status, worker_name, created_at);
            CREATE INDEX IF NOT EXISTS idx_logs_job ON job_logs(job_id, id);
            CREATE INDEX IF NOT EXISTS idx_worker_repos_worker ON worker_repos(worker_name, repo_alias);
            """
        )


def row_to_dict(row: sqlite3.Row | None) -> dict[str, Any] | None:
    return dict(row) if row else None


def get_job(conn: sqlite3.Connection, job_id: int, include_logs: bool = False) -> dict[str, Any] | None:
    job = row_to_dict(conn.execute("SELECT * FROM jobs WHERE id = ?", (job_id,)).fetchone())
    if job and include_logs:
        rows = conn.execute("SELECT * FROM job_logs WHERE job_id = ? ORDER BY id", (job_id,)).fetchall()
        job["logs"] = [dict(row) for row in rows]
    return job


def touch_worker(conn: sqlite3.Connection, name: str) -> None:
    ts = now_iso()
    conn.execute(
        """
        INSERT INTO workers (name, last_seen_at, created_at)
        VALUES (?, ?, ?)
        ON CONFLICT(name) DO UPDATE SET last_seen_at = excluded.last_seen_at
        """,
        (name, ts, ts),
    )
