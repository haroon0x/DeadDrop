from pydantic import BaseModel, Field


class JobCreate(BaseModel):
    title: str = Field(min_length=1, max_length=160)
    prompt: str = Field(min_length=1, max_length=20000)
    repo_alias: str = "default"
    worker_name: str = "local"


class LogCreate(BaseModel):
    stream: str = "system"
    content: str = Field(min_length=1, max_length=20000)


class CompleteJob(BaseModel):
    exit_code: int = 0
    final_summary: str = Field(default="", max_length=50000)
    receipt_json: str = Field(default="", max_length=50000)
    git_diff: str = Field(default="", max_length=500000)


class FailJob(BaseModel):
    exit_code: int = 1
    error_message: str = Field(default="", max_length=5000)
    final_summary: str = Field(default="", max_length=50000)
    receipt_json: str = Field(default="", max_length=50000)
    git_diff: str = Field(default="", max_length=500000)


class WorkerRepo(BaseModel):
    repo_alias: str = Field(min_length=1, max_length=80)
    display_name: str = Field(default="", max_length=160)


class WorkerRegister(BaseModel):
    worker_name: str = Field(default="local", min_length=1, max_length=80)
    repos: list[WorkerRepo] = Field(max_length=50)
