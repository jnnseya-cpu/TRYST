from fastapi.testclient import TestClient

from tryst_agents.app import app


def test_healthz():
    assert TestClient(app).get("/healthz").json() == {"status": "ok"}
