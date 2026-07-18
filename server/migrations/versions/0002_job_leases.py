from alembic import op
import sqlalchemy as sa

revision = "0002"
down_revision = "0001"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column("jobs", sa.Column("attempt_id", sa.String(length=36)))
    op.add_column("jobs", sa.Column("attempt_number", sa.Integer(), nullable=False, server_default="0"))
    op.add_column("jobs", sa.Column("lease_expires_at", sa.Text()))
    op.add_column("jobs", sa.Column("heartbeat_at", sa.Text()))
    op.add_column("jobs", sa.Column("cancel_requested_at", sa.Text()))
    op.create_index("idx_jobs_lease", "jobs", ["status", "lease_expires_at"])
    op.create_table(
        "job_attempts",
        sa.Column("attempt_id", sa.String(length=36), primary_key=True),
        sa.Column("job_id", sa.Integer(), sa.ForeignKey("jobs.id", ondelete="CASCADE"), nullable=False),
        sa.Column("attempt_number", sa.Integer(), nullable=False),
        sa.Column("worker_name", sa.Text(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False),
        sa.Column("started_at", sa.Text(), nullable=False),
        sa.Column("heartbeat_at", sa.Text(), nullable=False),
        sa.Column("lease_expires_at", sa.Text(), nullable=False),
        sa.Column("finished_at", sa.Text()),
        sa.Column("exit_code", sa.Integer()),
        sa.Column("error_message", sa.Text()),
    )
    op.create_index("idx_job_attempts_job", "job_attempts", ["job_id", "attempt_number"])
    op.add_column("job_logs", sa.Column("attempt_id", sa.String(length=36)))


def downgrade() -> None:
    op.drop_column("job_logs", "attempt_id")
    op.drop_index("idx_job_attempts_job", table_name="job_attempts")
    op.drop_table("job_attempts")
    op.drop_index("idx_jobs_lease", table_name="jobs")
    op.drop_column("jobs", "cancel_requested_at")
    op.drop_column("jobs", "heartbeat_at")
    op.drop_column("jobs", "lease_expires_at")
    op.drop_column("jobs", "attempt_number")
    op.drop_column("jobs", "attempt_id")
