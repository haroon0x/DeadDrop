from alembic import op
import sqlalchemy as sa

revision = "0001"
down_revision = None
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "workers",
        sa.Column("id", sa.Integer(), primary_key=True),
        sa.Column("name", sa.String(), nullable=False, unique=True),
        sa.Column("token_hash", sa.Text()),
        sa.Column("last_seen_at", sa.Text()),
        sa.Column("created_at", sa.Text(), nullable=False),
    )
    op.create_table(
        "jobs",
        sa.Column("id", sa.Integer(), primary_key=True),
        sa.Column("title", sa.String(length=160), nullable=False),
        sa.Column("prompt", sa.Text(), nullable=False),
        sa.Column("repo_alias", sa.Text(), nullable=False),
        sa.Column("worker_name", sa.Text(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),
        sa.Column("created_at", sa.Text(), nullable=False),
        sa.Column("updated_at", sa.Text(), nullable=False),
        sa.Column("started_at", sa.Text()),
        sa.Column("completed_at", sa.Text()),
        sa.Column("error_message", sa.Text()),
        sa.Column("final_summary", sa.Text()),
        sa.Column("receipt_json", sa.Text()),
        sa.Column("git_diff", sa.Text()),
        sa.Column("exit_code", sa.Integer()),
    )
    op.create_index("idx_jobs_status_worker", "jobs", ["status", "worker_name", "created_at"])
    op.create_table(
        "job_logs",
        sa.Column("id", sa.Integer(), primary_key=True),
        sa.Column("job_id", sa.Integer(), sa.ForeignKey("jobs.id", ondelete="CASCADE"), nullable=False),
        sa.Column("timestamp", sa.Text(), nullable=False),
        sa.Column("stream", sa.Text(), nullable=False),
        sa.Column("content", sa.Text(), nullable=False),
    )
    op.create_index("idx_logs_job", "job_logs", ["job_id", "id"])
    op.create_table(
        "worker_repos",
        sa.Column("id", sa.Integer(), primary_key=True),
        sa.Column("worker_name", sa.Text(), nullable=False),
        sa.Column("repo_alias", sa.Text(), nullable=False),
        sa.Column("display_name", sa.Text(), nullable=False),
        sa.Column("last_seen_at", sa.Text(), nullable=False),
        sa.Column("created_at", sa.Text(), nullable=False),
        sa.UniqueConstraint("worker_name", "repo_alias"),
    )
    op.create_index("idx_worker_repos_worker", "worker_repos", ["worker_name", "repo_alias"])


def downgrade() -> None:
    op.drop_index("idx_worker_repos_worker", table_name="worker_repos")
    op.drop_table("worker_repos")
    op.drop_index("idx_logs_job", table_name="job_logs")
    op.drop_table("job_logs")
    op.drop_index("idx_jobs_status_worker", table_name="jobs")
    op.drop_table("jobs")
    op.drop_table("workers")
