from pathlib import Path

from dotenv import load_dotenv
import logging

import psycopg
from flask import Flask, jsonify, render_template, request
from werkzeug.exceptions import HTTPException

load_dotenv(Path(__file__).resolve().parent.parent / ".env")
from monitor_pkg.config import SETTINGS
from monitor_pkg.db import SPOOL, ensure_schema
from . import queries_light as q

HOST = SETTINGS.dashboard_host
PORT = SETTINGS.dashboard_port
app = Flask(__name__)
log = logging.getLogger(__name__)
WINDOWS = {"15 minutes", "1 hour", "6 hours", "24 hours", "7 days", "30 days"}
WINDOW_BUCKET_SECONDS = {
    "15 minutes": 10,
    "1 hour": 10,
    "6 hours": 60,
    "24 hours": 300,
    "7 days": 1800,
    "30 days": 7200,
}


def window() -> str:
    value = request.args.get("window", "1 hour")
    if value not in WINDOWS:
        raise ValueError("window must be one of: " + ", ".join(sorted(WINDOWS)))
    return value


def requested_window() -> tuple[str, int]:
    value = window()
    return value, WINDOW_BUCKET_SECONDS[value]


@app.errorhandler(ValueError)
def invalid_request(error):
    return jsonify({"error": str(error)}), 400


@app.errorhandler(psycopg.Error)
def database_error(error):
    log.warning("dashboard database error: %s", error)
    return jsonify({"error": "database unavailable"}), 503


@app.errorhandler(Exception)
def unexpected_error(error):
    if isinstance(error, HTTPException):
        return error
    log.exception("dashboard request failed")
    return jsonify({"error": "internal server error"}), 500

@app.after_request
def no_store(resp):
    # APIs não devem ser cacheadas; HTML pode ser cacheado levemente
    if request.path.startswith('/api/'):
        resp.headers['Cache-Control'] = 'no-store, max-age=0'
    return resp

@app.route("/")
def index():
    return render_template("index.html")

@app.route("/api/cpu")
def api_cpu():
    value, bucket = requested_window()
    return jsonify(q.q_cpu(value, bucket))

@app.route("/api/memory")
def api_memory():
    value, bucket = requested_window()
    return jsonify(q.q_memory(value, bucket))

@app.route("/api/gpu")
def api_gpu():
    value, bucket = requested_window()
    return jsonify(q.q_gpu(value, bucket))

@app.route("/api/sensors/cpu_temps")
def api_cpu_temps():
    value, bucket = requested_window()
    return jsonify(q.q_cpu_temps(value, bucket))

@app.route("/api/sensors/cpu_temps_latest")
def api_cpu_temps_latest():
    return jsonify(q.q_cpu_temps_latest())

@app.route("/api/sensors/latest")
def api_sensors_latest():
    return jsonify(q.q_sensors_latest())


@app.route("/api/sensors/history")
def api_sensors_history():
    value, bucket = requested_window()
    sensor_type = request.args.get("type", "power").strip()
    if not sensor_type or len(sensor_type) > 40:
        raise ValueError("sensor type must contain between 1 and 40 characters")
    return jsonify(q.q_sensors_history(value, bucket, sensor_type))

@app.route("/api/disk/usage")
def api_disk_usage():
    return jsonify(q.q_disk_usage())


@app.route("/api/disk/usage/history")
def api_disk_usage_history():
    value, bucket = requested_window()
    return jsonify(q.q_disk_usage_history(value, bucket))

@app.route("/api/disk/physical")
def api_physical():
    return jsonify(q.q_physical_disk())

@app.route("/api/disk/smart")
def api_smart():
    return jsonify(q.q_disk_smart_latest())

@app.route("/api/disk/smart/history")
def api_smart_history():
    value, bucket = requested_window()
    return jsonify(q.q_disk_smart_history(value, bucket))

@app.route("/api/disk/io")
def api_disk_io():
    value, bucket = requested_window()
    return jsonify(q.q_disk_io(value, bucket))

@app.route("/api/net")
def api_net():
    value, bucket = requested_window()
    return jsonify(q.q_net(value, bucket))

@app.route("/api/net/latest")
def api_net_latest():
    return jsonify(q.q_net_latest())


@app.route("/api/power")
def api_power():
    value, bucket = requested_window()
    return jsonify(q.q_power(value, bucket))

@app.route("/api/processes")
def api_processes():
    return jsonify(q.q_processes())

@app.route("/api/system")
def api_system():
    return jsonify(q.q_system())

@app.route("/api/heartbeat")
def api_heartbeat():
    return jsonify(q.q_heartbeat())

@app.route("/api/db_size")
def api_db_size():
    return jsonify(q.q_db_size())

@app.route("/api/health")
def health():
    return jsonify({"status": "ok"})

@app.route("/api/ready")
def ready():
    ensure_schema()
    return jsonify({"status": "ready"})

@app.route("/api/status")
def status():
    return jsonify(SPOOL.status())

if __name__ == "__main__":
    app.run(host=HOST, port=PORT, debug=False)
