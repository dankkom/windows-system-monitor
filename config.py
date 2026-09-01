import logging
import os
import socket
from dataclasses import dataclass
from pathlib import Path
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from dotenv import load_dotenv

BASE_DIR = Path(__file__).parent
load_dotenv(BASE_DIR / ".env")


def _positive_int(name: str, default: int) -> int:
    value = int(os.getenv(name, str(default)))
    if value <= 0:
        raise ValueError(f"{name} must be positive")
    return value


def _nonnegative_float(name: str, default: float | None = None) -> float | None:
    raw = os.getenv(name, "" if default is None else str(default)).strip()
    if not raw:
        return None
    try:
        value = float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc
    if value < 0:
        raise ValueError(f"{name} must be non-negative")
    return value


@dataclass(frozen=True)
class Settings:
    database_url: str
    hostname: str
    connect_timeout: int
    retry_seconds: int
    buffer_max_bytes: int
    dashboard_host: str
    dashboard_port: int
    dashboard_timezone: str
    power_aux_baseline_w: float
    power_psu_efficiency: float
    power_gpu_idle_w: float | None
    power_gpu_max_w: float | None
    log_level: int


def load_settings() -> Settings:
    database_url = os.getenv("DATABASE_URL", "").strip()
    if not database_url.startswith(("postgresql://", "postgres://")):
        raise ValueError("DATABASE_URL must be a PostgreSQL URL")
    port = _positive_int("DASHBOARD_PORT", 8501)
    if port > 65535:
        raise ValueError("DASHBOARD_PORT must be <= 65535")
    timezone = os.getenv("DASHBOARD_TIMEZONE", "America/Sao_Paulo").strip()
    try:
        ZoneInfo(timezone)
    except ZoneInfoNotFoundError as exc:
        raise ValueError("DASHBOARD_TIMEZONE must be a valid IANA timezone") from exc
    baseline = _nonnegative_float("POWER_AUX_BASELINE_W", 30.0)
    efficiency = _nonnegative_float("POWER_PSU_EFFICIENCY", 0.90)
    if efficiency is None or not 0 < efficiency <= 1:
        raise ValueError("POWER_PSU_EFFICIENCY must be greater than 0 and at most 1")
    gpu_idle = _nonnegative_float("POWER_GPU_IDLE_W")
    gpu_max = _nonnegative_float("POWER_GPU_MAX_W")
    if (gpu_idle is None) != (gpu_max is None):
        raise ValueError("POWER_GPU_IDLE_W and POWER_GPU_MAX_W must be configured together")
    if gpu_idle is not None and gpu_max is not None and gpu_max < gpu_idle:
        raise ValueError("POWER_GPU_MAX_W must be greater than or equal to POWER_GPU_IDLE_W")
    level_name = os.getenv("LOG_LEVEL", "INFO").upper()
    level = getattr(logging, level_name, None)
    if not isinstance(level, int):
        raise ValueError("LOG_LEVEL must be a standard logging level")
    return Settings(
        database_url=database_url,
        hostname=os.getenv("HOSTNAME") or socket.gethostname(),
        connect_timeout=_positive_int("DATABASE_CONNECT_TIMEOUT", 10),
        retry_seconds=_positive_int("DATABASE_RETRY_SECONDS", 30),
        buffer_max_bytes=_positive_int("BUFFER_MAX_BYTES", 2 * 1024**3),
        dashboard_host=os.getenv("DASHBOARD_HOST", "127.0.0.1"),
        dashboard_port=port,
        dashboard_timezone=timezone,
        power_aux_baseline_w=baseline or 0.0,
        power_psu_efficiency=efficiency,
        power_gpu_idle_w=gpu_idle,
        power_gpu_max_w=gpu_max,
        log_level=level,
    )


SETTINGS = load_settings()
DATABASE_URL = SETTINGS.database_url
HOSTNAME = SETTINGS.hostname
LOG_LEVEL = logging.getLevelName(SETTINGS.log_level)
LOG_DIR = BASE_DIR / "logs"
LOG_DIR.mkdir(exist_ok=True)
BUFFER_PATH = LOG_DIR / "pending_batches.sqlite3"

INTERVALS = {name: _positive_int(env, default) for name, env, default in (
    ("cpu", "INTERVAL_CPU", 10), ("memory", "INTERVAL_MEMORY", 10),
    ("disk_io", "INTERVAL_DISK_IO", 10), ("disk_usage", "INTERVAL_DISK_USAGE", 60),
    ("disk_physical", "INTERVAL_DISK_PHYSICAL", 300), ("disk_smart", "INTERVAL_DISK_SMART", 300),
    ("network", "INTERVAL_NETWORK", 10), ("gpu", "INTERVAL_GPU", 10),
    ("sensors", "INTERVAL_SENSORS", 15), ("processes", "INTERVAL_PROCESSES", 30),
    ("connections", "INTERVAL_CONNECTIONS", 30), ("services", "INTERVAL_SERVICES", 60),
    ("system", "INTERVAL_SYSTEM", 60), ("eventlog", "INTERVAL_EVENTLOG", 60),
)}
TOP_PROCESSES = _positive_int("TOP_PROCESSES", 50)
