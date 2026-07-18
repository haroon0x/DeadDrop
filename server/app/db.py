import json
import os
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any
from uuid import uuid4

from alembic import command
from alembic.config import Config
from sqlalchemy import (
    Column,
    ForeignKey,
    Index,
    Integer,
    MetaData,
    String,
    Table,
    Text,
    UniqueConstraint,
    create_engine,
    delete,
    desc,
    insert,
    select,
    text,
    update,
)
from sqlalchemy.engine import Connection, Engine

metadata = MetaData()

workers = Table(
    "workers",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("name", String, unique=True, nullable=False),
    Column("token_hash", Text),
    Column("last_seen_at", Text),
    Column("created_at", Text, nullable=False),
)

jobs = Table(
    "jobs",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("title", String(160), nullable=False),
    Column("prompt", Text, nullable=False),
    Column("repo_alias", Text, nullable=False, default="default"),
    Column("worker_name", Text, nullable=False, default="local"),
    Column("status", Text, nullable=False),
    Column("created_at", Text, nullable=False),
    Column("updated_at", Text, nullable=False),
    Column("started_at", Text),
    Column("completed_at", Text),
    Column("attempt_id", String(36)),
    Column("attempt_number", Integer, nullable=False, default=0),
    Column("lease_expires_at", Text),
    Column("heartbeat_at", Text),
    Column("cancel_requested_at", Text),
    Column("error_message", Text),
    Column("final_summary", Text),
    Column("receipt_json", Text),
    Column("git_diff", Text),
    Column("exit_code", Integer),
    Index("idx_jobs_status_worker", "status", "worker_name", "created_at"),
    Index("idx_jobs_lease", "status", "lease_expires_at"),
)

job_attempts = Table(
    "job_attempts",
    metadata,
    Column("attempt_id", String(36), primary_key=True),
    Column("job_id", Integer, ForeignKey("jobs.id", ondelete="CASCADE"), nullable=False),
    Column("attempt_number", Integer, nullable=False),
    Column("worker_name", Text, nullable=False),
    Column("status", Text, nullable=False),
    Column("started_at", Text, nullable=False),
    Column("heartbeat_at", Text, nullable=False),
    Column("lease_expires_at", Text, nullable=False),
    Column("finished_at", Text),
    Column("exit_code", Integer),
    Column("error_message", Text),
    Index("idx_job_attempts_job", "job_id", "attempt_number"),
)

job_logs = Table(
    "job_logs",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("job_id", Integer, ForeignKey("jobs.id", ondelete="CASCADE"), nullable=False),
    Column("timestamp", Text, nullable=False),
    Column("attempt_id", String(36)),
    Column("stream", Text, nullable=False),
    Column("content", Text, nullable=False),
    Index("idx_logs_job", "job_id", "id"),
)

worker_repos = Table(
    "worker_repos",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("worker_name", Text, nullable=False),
    Column("repo_alias", Text, nullable=False),
    Column("display_name", Text, nullable=False),
    Column("last_seen_at", Text, nullable=False),
    Column("created_at", Text, nullable=False),
    UniqueConstraint("worker_name", "repo_alias"),
    Index("idx_worker_repos_worker", "worker_name", "repo_alias"),
)

_engine: Engine | None = None
_engine_url: str | None = None


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def lease_expires_iso(seconds: int = 60) -> str:
    return (datetime.now(timezone.utc) + timedelta(seconds=seconds)).isoformat(timespec="seconds")


def database_url() -> str:
    url = os.getenv("DATABASE_URL")
    if url:
        if url.startswith("postgres://"):
            return "postgresql+psycopg://" + url.removeprefix("postgres://")
        if url.startswith("postgresql://"):
            return "postgresql+psycopg://" + url.removeprefix("postgresql://")
        return url
    path = "./deaddrop.db"
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    return f"sqlite:///{path}"


def engine() -> Engine:
    global _engine, _engine_url
    url = database_url()
    if _engine is None or _engine_url != url:
        kwargs: dict[str, Any] = {"pool_pre_ping": True}
        if url.startswith("sqlite"):
            kwargs["connect_args"] = {"check_same_thread": False}
        _engine = create_engine(url, **kwargs)
        _engine_url = url
    return _engine


@contextmanager
def connect():
    with engine().begin() as conn:
        yield conn


def init_db() -> None:
    config = migration_config()
    with engine().connect() as conn:
        tables = set(conn.dialect.get_table_names(conn))
    if "jobs" in tables and "alembic_version" not in tables:
        command.stamp(config, "0001")
    command.upgrade(config, "head")


def migration_config() -> Config:
    server_dir = Path(__file__).resolve().parents[1]
    config = Config(server_dir / "alembic.ini")
    config.set_main_option("script_location", str(server_dir / "migrations"))
    config.set_main_option("sqlalchemy.url", database_url().replace("%", "%%"))
    return config


def reset_engine_for_tests() -> None:
    global _engine, _engine_url
    if _engine is not None:
        _engine.dispose()
    _engine = None
    _engine_url = None


def row_to_dict(row: Any | None) -> dict[str, Any] | None:
    if not row:
        return None
    data = dict(row)
    data["receipt"] = None
    raw = data.get("receipt_json")
    if raw:
        try:
            data["receipt"] = json.loads(raw)
        except json.JSONDecodeError:
            data["receipt"] = None
    return data


def list_jobs(conn: Connection) -> list[dict[str, Any]]:
    rows = conn.execute(select(jobs).order_by(desc(jobs.c.created_at), desc(jobs.c.id))).mappings().all()
    return [dict(row) for row in rows]


def list_worker_repos(conn: Connection) -> list[dict[str, Any]]:
    rows = conn.execute(
        select(
            worker_repos.c.worker_name,
            worker_repos.c.repo_alias,
            worker_repos.c.display_name,
            worker_repos.c.last_seen_at,
        ).order_by(worker_repos.c.worker_name, worker_repos.c.repo_alias)
    ).mappings().all()
    return [dict(row) for row in rows]


def get_job(
    conn: Connection,
    job_id: int,
    include_logs: bool = False,
    log_limit: int = 200,
    before_log_id: int | None = None,
) -> dict[str, Any] | None:
    job = row_to_dict(conn.execute(select(jobs).where(jobs.c.id == job_id)).mappings().first())
    if job and include_logs:
        log_limit = max(1, min(log_limit, 500))
        stmt = select(job_logs).where(job_logs.c.job_id == job_id)
        if before_log_id:
            stmt = stmt.where(job_logs.c.id < before_log_id)
        rows = conn.execute(stmt.order_by(desc(job_logs.c.id)).limit(log_limit)).mappings().all()
        logs = [dict(row) for row in reversed(rows)]
        job["logs"] = logs
        if logs:
            oldest_id = logs[0]["id"]
            job["older_logs_available"] = (
                conn.execute(
                    select(job_logs.c.id)
                    .where(job_logs.c.job_id == job_id, job_logs.c.id < oldest_id)
                    .limit(1)
                ).first()
                is not None
            )
            job["oldest_log_id"] = oldest_id
        else:
            job["older_logs_available"] = False
            job["oldest_log_id"] = None
    return job


def create_job_record(conn: Connection, values: dict[str, Any]) -> int:
    result = conn.execute(insert(jobs).values(**values).returning(jobs.c.id))
    return int(result.scalar_one())


def claim_next_job(conn: Connection, worker_name: str, ts: str) -> dict[str, Any] | None:
    recover_stale_jobs(conn, ts)
    row = conn.execute(
        select(jobs.c.id, jobs.c.attempt_number)
        .where(jobs.c.status == "queued", jobs.c.worker_name == worker_name)
        .order_by(jobs.c.created_at, jobs.c.id)
        .limit(1)
        .with_for_update(skip_locked=True)
    ).first()
    if not row:
        return None
    job_id = int(row.id)
    attempt_id = str(uuid4())
    attempt_number = int(row.attempt_number or 0) + 1
    lease_expires_at = lease_expires_iso()
    result = conn.execute(
        update(jobs)
        .where(jobs.c.id == job_id, jobs.c.status == "queued")
        .values(
            status="running",
            started_at=ts,
            updated_at=ts,
            attempt_id=attempt_id,
            attempt_number=attempt_number,
            heartbeat_at=ts,
            lease_expires_at=lease_expires_at,
            cancel_requested_at=None,
            completed_at=None,
        )
    )
    if result.rowcount != 1:
        return None
    conn.execute(
        insert(job_attempts).values(
            attempt_id=attempt_id,
            job_id=job_id,
            attempt_number=attempt_number,
            worker_name=worker_name,
            status="running",
            started_at=ts,
            heartbeat_at=ts,
            lease_expires_at=lease_expires_at,
        )
    )
    return get_job(conn, job_id, include_logs=False)


def recover_stale_jobs(conn: Connection, ts: str) -> int:
    stale = conn.execute(
        select(jobs.c.id, jobs.c.attempt_id, jobs.c.cancel_requested_at)
        .where(jobs.c.status == "running", jobs.c.lease_expires_at < ts)
    ).all()
    recovered = 0
    for row in stale:
        cancelled = row.cancel_requested_at is not None
        status = "cancelled" if cancelled else "queued"
        values: dict[str, Any] = {
            "status": status,
            "updated_at": ts,
            "attempt_id": None,
            "heartbeat_at": None,
            "lease_expires_at": None,
        }
        if cancelled:
            values["completed_at"] = ts
        result = conn.execute(
            update(jobs)
            .where(jobs.c.id == row.id, jobs.c.status == "running", jobs.c.attempt_id == row.attempt_id)
            .values(**values)
        )
        if result.rowcount != 1:
            continue
        conn.execute(
            update(job_attempts)
            .where(job_attempts.c.attempt_id == row.attempt_id)
            .values(
                status="cancelled" if cancelled else "lost",
                finished_at=ts,
                error_message="Worker lease expired",
            )
        )
        recovered += 1
    return recovered


def touch_worker(conn: Connection, name: str) -> None:
    ts = now_iso()
    dialect = conn.dialect.name
    if dialect == "sqlite":
        conn.execute(
            text(
                """
                INSERT INTO workers (name, last_seen_at, created_at)
                VALUES (:name, :last_seen_at, :created_at)
                ON CONFLICT(name) DO UPDATE SET last_seen_at = excluded.last_seen_at
                """
            ),
            {"name": name, "last_seen_at": ts, "created_at": ts},
        )
        return
    conn.execute(
        text(
            """
            INSERT INTO workers (name, last_seen_at, created_at)
            VALUES (:name, :last_seen_at, :created_at)
            ON CONFLICT(name) DO UPDATE SET last_seen_at = excluded.last_seen_at
            """
        ),
        {"name": name, "last_seen_at": ts, "created_at": ts},
    )


def upsert_worker_repo(conn: Connection, worker_name: str, repo_alias: str, display_name: str, ts: str) -> None:
    conn.execute(
        text(
            """
            INSERT INTO worker_repos (worker_name, repo_alias, display_name, last_seen_at, created_at)
            VALUES (:worker_name, :repo_alias, :display_name, :last_seen_at, :created_at)
            ON CONFLICT(worker_name, repo_alias)
            DO UPDATE SET display_name = excluded.display_name, last_seen_at = excluded.last_seen_at
            """
        ),
        {
            "worker_name": worker_name,
            "repo_alias": repo_alias,
            "display_name": display_name,
            "last_seen_at": ts,
            "created_at": ts,
        },
    )


def delete_all_for_tests() -> None:
    with connect() as conn:
        conn.execute(delete(job_logs))
        conn.execute(delete(job_attempts))
        conn.execute(delete(jobs))
        conn.execute(delete(worker_repos))
        conn.execute(delete(workers))
