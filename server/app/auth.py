import os
from secrets import compare_digest

from fastapi import Header, HTTPException, Request

def _bearer(authorization: str | None) -> str | None:
    if not authorization:
        return None
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return None
    return token


def owner_token() -> str:
    token = os.getenv("OWNER_TOKEN")
    if not token:
        raise RuntimeError("OWNER_TOKEN is required")
    return token


def worker_token() -> str:
    token = os.getenv("WORKER_TOKEN")
    if not token:
        raise RuntimeError("WORKER_TOKEN is required")
    return token


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
    owner_token()
    worker_token()
