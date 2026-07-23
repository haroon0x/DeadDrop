from alembic import op
import sqlalchemy as sa

revision = "0004"
down_revision = "0003"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column("jobs", sa.Column("agent", sa.Text()))


def downgrade() -> None:
    op.drop_column("jobs", "agent")
