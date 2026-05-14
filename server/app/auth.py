import os

from fastapi import Header, HTTPException, Request


def _bearer(authorization: str | None) -> str | None:
    if not authorization:
        return None
    scheme, _, token = authorization.partition(" ")
    if scheme.lower() != "bearer" or not token:
        return None
    return token


def owner_token() -> str:
    return os.getenv("OWNER_TOKEN", "owner_dev")


def worker_token() -> str:
    return os.getenv("WORKER_TOKEN", "worker_dev")


def require_owner(authorization: str | None = Header(default=None)) -> None:
    if _bearer(authorization) != owner_token():
        raise HTTPException(status_code=401, detail="Invalid owner token")


def require_worker(authorization: str | None = Header(default=None)) -> None:
    if _bearer(authorization) != worker_token():
        raise HTTPException(status_code=401, detail="Invalid worker token")


def owner_from_request(request: Request) -> bool:
    token = request.cookies.get("owner_token") or request.query_params.get("token")
    return token == owner_token()
