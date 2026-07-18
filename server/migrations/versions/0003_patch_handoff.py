from alembic import op
import sqlalchemy as sa

revision = "0003"
down_revision = "0002"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column("jobs", sa.Column("baseline_commit", sa.String(length=64)))


def downgrade() -> None:
    op.drop_column("jobs", "baseline_commit")
