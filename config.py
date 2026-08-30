import os
import socket
from pathlib import Path
from dotenv import load_dotenv

BASE_DIR = Path(__file__).parent
load_dotenv(BASE_DIR / ".env")

DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://postgres:22932293@localhost:5432/system_monitor")
HOSTNAME = os.getenv("HOSTNAME") or socket.gethostname()

def _int(env, default):
    try:
        return int(os.getenv(env, str(default)))
    except:
        return default

INTERVALS = {
    "cpu": _int("INTERVAL_CPU", 10),
    "memory": _int("INTERVAL_MEMORY", 10),
    "disk_io": _int("INTERVAL_DISK_IO", 10),
    "disk_usage": _int("INTERVAL_DISK_USAGE", 60),
    "disk_physical": _int("INTERVAL_DISK_PHYSICAL", 300),
    "disk_smart": _int("INTERVAL_DISK_SMART", 300),
    "network": _int("INTERVAL_NETWORK", 10),
    "gpu": _int("INTERVAL_GPU", 10),
    "sensors": _int("INTERVAL_SENSORS", 15),
    "processes": _int("INTERVAL_PROCESSES", 30),
    "connections": _int("INTERVAL_CONNECTIONS", 30),
    "services": _int("INTERVAL_SERVICES", 60),
    "system": _int("INTERVAL_SYSTEM", 60),
    "eventlog": _int("INTERVAL_EVENTLOG", 60),
}

TOP_PROCESSES = _int("TOP_PROCESSES", 50)
LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO").upper()
LOG_DIR = BASE_DIR / "logs"
LOG_DIR.mkdir(exist_ok=True)
