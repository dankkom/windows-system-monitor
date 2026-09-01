from flask import Flask, jsonify, render_template, request
try:
    import queries_light as q
except ImportError:
    from . import queries_light as q

app = Flask(__name__)

@app.route("/")
def index():
    return render_template("index.html")

@app.route("/api/cpu")
def api_cpu():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_cpu(window))

@app.route("/api/memory")
def api_memory():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_memory(window))

@app.route("/api/gpu")
def api_gpu():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_gpu(window))

@app.route("/api/sensors/cpu_temps")
def api_cpu_temps():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_cpu_temps(window))

@app.route("/api/sensors/cpu_temps_latest")
def api_cpu_temps_latest():
    return jsonify(q.q_cpu_temps_latest())

@app.route("/api/sensors/latest")
def api_sensors_latest():
    return jsonify(q.q_sensors_latest())

@app.route("/api/disk/usage")
def api_disk_usage():
    return jsonify(q.q_disk_usage())

@app.route("/api/disk/physical")
def api_physical():
    return jsonify(q.q_physical_disk())

@app.route("/api/disk/smart")
def api_smart():
    return jsonify(q.q_disk_smart_latest())

@app.route("/api/disk/io")
def api_disk_io():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_disk_io(window))

@app.route("/api/net")
def api_net():
    window = request.args.get("window", "1 hour")
    return jsonify(q.q_net(window))

@app.route("/api/net/latest")
def api_net_latest():
    return jsonify(q.q_net_latest())

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

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8501, debug=False)
