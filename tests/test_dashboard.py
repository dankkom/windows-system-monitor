import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from dashboard import app as dashboard_app

app = dashboard_app.app


def test_health_is_live_without_database():
    response = app.test_client().get("/api/health")
    assert response.status_code == 200
    assert response.json == {"status": "ok"}


def test_window_is_validated_before_query():
    response = app.test_client().get("/api/cpu?window=bad")
    assert response.status_code == 400


def test_http_not_found_is_not_converted_to_server_error():
    assert app.test_client().get("/missing").status_code == 404


def test_power_route_uses_server_bucket(monkeypatch):
    called = {}

    def fake_power(window, bucket):
        called.update(window=window, bucket=bucket)
        return {"series": []}

    monkeypatch.setattr(dashboard_app.q, "q_power", fake_power)
    response = app.test_client().get("/api/power?window=24 hours")
    assert response.status_code == 200
    assert called == {"window": "24 hours", "bucket": 300}


def test_sensor_type_is_bounded_before_query():
    response = app.test_client().get("/api/sensors/history?type=" + "x" * 41)
    assert response.status_code == 400
