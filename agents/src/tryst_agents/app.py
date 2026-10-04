"""Agent service entry point. Each agent will deploy separately (03 §2.2); this
scaffold exposes health and a Broker eligibility endpoint for integration tests."""

from __future__ import annotations

from fastapi import FastAPI

app = FastAPI(title="tryst-agents", version="0.1.0", docs_url=None, redoc_url=None)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}
