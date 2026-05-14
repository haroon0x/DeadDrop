from pydantic import BaseModel, Field


class JobCreate(BaseModel):
    title: str = Field(min_length=1, max_length=160)
    prompt: str = Field(min_length=1)
    repo_alias: str = "default"
    worker_name: str = "local"


class LogCreate(BaseModel):
    stream: str = "system"
    content: str = Field(min_length=1)


class CompleteJob(BaseModel):
    exit_code: int = 0
    final_summary: str = ""
    git_diff: str = ""


class FailJob(BaseModel):
    exit_code: int = 1
    error_message: str = ""
    final_summary: str = ""
    git_diff: str = ""


class WorkerRepo(BaseModel):
    repo_alias: str
    display_name: str = ""


class WorkerRegister(BaseModel):
    worker_name: str = "local"
    repos: list[WorkerRepo]
