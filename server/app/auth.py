import os
from secrets import compare_digest

from fastapi import Header, HTTPException, Request

DEFAULT_OWNER_TOKEN = "owner_dev"
DEFAULT_WORKER_TOKEN = "worker_dev"


def _bearer(authorization: str | None) -> str | None:
    if not authorization:
        return None
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return None
    return token


def owner_token() -> str:
    return os.getenv("OWNER_TOKEN", DEFAULT_OWNER_TOKEN)


def worker_token() -> str:
    return os.getenv("WORKER_TOKEN", DEFAULT_WORKER_TOKEN)


def require_owner(authorization: str | None = Header(default=None)) -> None:
    token = _bearer(authorization)
    if token is None or not compare_digest(token, owner_token()):
        raise HTTPException(status_code=401, detail="Invalid owner token")


def require_worker(authorization: str | None = Header(default=None)) -> None:
    token = _bearer(authorization)
    if token is None or not compare_digest(token, worker_token()):
        raise HTTPException(status_code=401, detail="Invalid worker token")


def owner_from_request(request: Request) -> bool:
    token = request.cookies.get("owner_token")
    return token is not None and compare_digest(token, owner_token())


def secure_cookies() -> bool:
    return os.getenv("SECURE_COOKIES", "").lower() in {"1", "true", "yes"}


def validate_auth_config() -> None:
    database_url = os.getenv("DATABASE_URL", "")
    production_db = database_url.startswith(("postgres://", "postgresql://"))
    production_mode = secure_cookies() or production_db
    if not production_mode:
        return
    if owner_token() == DEFAULT_OWNER_TOKEN or worker_token() == DEFAULT_WORKER_TOKEN:
        raise RuntimeError(
            "Production requires non-default OWNER_TOKEN and WORKER_TOKEN. "
            "Set strong secrets in the host environment."
        )
