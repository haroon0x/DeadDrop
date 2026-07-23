from pydantic import BaseModel, Field, field_validator

from . import models


class JobCreate(BaseModel):
    title: str = Field(min_length=1, max_length=160)
    prompt: str = Field(min_length=1, max_length=20000)
    repo_alias: str = "default"
    worker_name: str = "local"
    agent: str = models.DEFAULT_AGENT

    @field_validator("agent")
    @classmethod
    def known_agent(cls, value: str) -> str:
        value = value.strip()
        if value not in models.AGENTS:
            allowed = ", ".join(sorted(name for name in models.AGENTS if name))
            raise ValueError(f"unknown agent {value!r}; allowed: {allowed}")
        return value

    @field_validator("repo_alias", "worker_name")
    @classmethod
    def non_empty(cls, value: str) -> str:
        value = value.strip()
        if not value:
            raise ValueError("must not be blank")
        return value


class LogCreate(BaseModel):
    attempt_id: str = Field(min_length=36, max_length=36)
    stream: str = "system"
    content: str = Field(min_length=1, max_length=20000)


class CompleteJob(BaseModel):
    attempt_id: str = Field(min_length=36, max_length=36)
    exit_code: int = 0
    final_summary: str = Field(default="", max_length=50000)
    receipt_json: str = Field(default="", max_length=50000)
    git_diff: str = Field(default="", max_length=500000)
    baseline_commit: str = Field(default="", max_length=64)


class FailJob(BaseModel):
    attempt_id: str = Field(min_length=36, max_length=36)
    exit_code: int = 1
    error_message: str = Field(default="", max_length=5000)
    final_summary: str = Field(default="", max_length=50000)
    receipt_json: str = Field(default="", max_length=50000)
    git_diff: str = Field(default="", max_length=500000)
    baseline_commit: str = Field(default="", max_length=64)


class AttemptRequest(BaseModel):
    attempt_id: str = Field(min_length=36, max_length=36)


class CancelledJob(BaseModel):
    attempt_id: str = Field(min_length=36, max_length=36)
    exit_code: int = 130
    final_summary: str = Field(default="", max_length=50000)
    receipt_json: str = Field(default="", max_length=50000)
    git_diff: str = Field(default="", max_length=500000)
    baseline_commit: str = Field(default="", max_length=64)


class WorkerRepo(BaseModel):
    repo_alias: str = Field(min_length=1, max_length=80)
    display_name: str = Field(default="", max_length=160)


class WorkerRegister(BaseModel):
    worker_name: str = Field(default="local", min_length=1, max_length=80)
    repos: list[WorkerRepo] = Field(max_length=50)
