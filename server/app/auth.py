import os
import secrets
from secrets import compare_digest

from fastapi import Form, Header, HTTPException, Request

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


def generate_csrf_token() -> str:
    return secrets.token_urlsafe(32)


def verify_csrf_token(request: Request, csrf_token: str = Form(None)) -> None:
    # 1. Get token from cookie
    cookie_token = request.cookies.get("csrf_token")
    # 2. Get token from form or header
    header_token = request.headers.get("X-CSRF-Token")
    provided_token = csrf_token or header_token
    
    if not cookie_token or not provided_token or not compare_digest(cookie_token, provided_token):
        raise HTTPException(status_code=403, detail="CSRF token validation failed")


def secure_cookies() -> bool:
    # Default to True for production security. 
    # Only allow False if explicitly requested (for local dev without HTTPS).
    return os.getenv("SECURE_COOKIES", "true").lower() not in {"0", "false", "no"}


def validate_auth_config() -> None:

    # Fail fast if tokens are missing in production
    if not os.getenv("OWNER_TOKEN") or not os.getenv("WORKER_TOKEN"):
        if os.getenv("DATABASE_URL"): # Sign of production
            raise RuntimeError("OWNER_TOKEN and WORKER_TOKEN must be set in production")
    owner_token()
    worker_token()
