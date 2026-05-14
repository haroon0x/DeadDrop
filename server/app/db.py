import os
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

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
    Column("error_message", Text),
    Column("final_summary", Text),
    Column("git_diff", Text),
    Column("exit_code", Integer),
    Index("idx_jobs_status_worker", "status", "worker_name", "created_at"),
)

job_logs = Table(
    "job_logs",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("job_id", Integer, ForeignKey("jobs.id", ondelete="CASCADE"), nullable=False),
    Column("timestamp", Text, nullable=False),
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


def database_url() -> str:
    url = os.getenv("DATABASE_URL")
    if url:
        if url.startswith("postgres://"):
            return "postgresql+psycopg://" + url.removeprefix("postgres://")
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
    metadata.create_all(engine())


def reset_engine_for_tests() -> None:
    global _engine, _engine_url
    if _engine is not None:
        _engine.dispose()
    _engine = None
    _engine_url = None


def row_to_dict(row: Any | None) -> dict[str, Any] | None:
    return dict(row) if row else None


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
    row = conn.execute(
        select(jobs.c.id)
        .where(jobs.c.status == "queued", jobs.c.worker_name == worker_name)
        .order_by(jobs.c.created_at, jobs.c.id)
        .limit(1)
        .with_for_update(skip_locked=True)
    ).first()
    if not row:
        return None
    job_id = int(row.id)
    conn.execute(
        update(jobs)
        .where(jobs.c.id == job_id, jobs.c.status == "queued")
        .values(status="running", started_at=ts, updated_at=ts)
    )
    return get_job(conn, job_id, include_logs=True)


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
        conn.execute(delete(jobs))
        conn.execute(delete(worker_repos))
        conn.execute(delete(workers))
