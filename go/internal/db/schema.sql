-- Schema system_monitor - máximo de dados coletados
-- PostgreSQL 18, compatível com TimescaleDB (opcional)

CREATE SCHEMA IF NOT EXISTS monitor;

-- 1. CPU
CREATE TABLE IF NOT EXISTS monitor.cpu (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    cpu_total_percent REAL,
    cpu_per_core_percent REAL[],
    cpu_count_logical INT,
    cpu_count_physical INT,
    freq_current_mhz REAL,
    freq_min_mhz REAL,
    freq_max_mhz REAL,
    load_1m REAL,
    ctx_switches BIGINT,
    interrupts BIGINT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_cpu_ts ON monitor.cpu (ts DESC);
CREATE INDEX IF NOT EXISTS idx_cpu_hostname_ts ON monitor.cpu (hostname, ts DESC);

-- 2. Memory
CREATE TABLE IF NOT EXISTS monitor.memory (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    total_bytes BIGINT,
    available_bytes BIGINT,
    used_bytes BIGINT,
    used_percent REAL,
    free_bytes BIGINT,
    active_bytes BIGINT,
    inactive_bytes BIGINT,
    cached_bytes BIGINT,
    wired_bytes BIGINT,
    buffers_bytes BIGINT,
    shared_bytes BIGINT,
    slab_bytes BIGINT,
    swap_total_bytes BIGINT,
    swap_used_bytes BIGINT,
    swap_free_bytes BIGINT,
    swap_used_percent REAL,
    swap_sin BIGINT,
    swap_sout BIGINT,
    pagefile_total BIGINT,
    pagefile_used BIGINT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_memory_ts ON monitor.memory (ts DESC);

-- 3. Disk Usage (por volume)
CREATE TABLE IF NOT EXISTS monitor.disk_usage (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    device TEXT NOT NULL,
    mountpoint TEXT,
    fstype TEXT,
    total_bytes BIGINT,
    used_bytes BIGINT,
    free_bytes BIGINT,
    used_percent REAL,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_disk_usage_ts ON monitor.disk_usage (ts DESC);
CREATE INDEX IF NOT EXISTS idx_disk_usage_device ON monitor.disk_usage (device, ts DESC);

-- 4. Disk IO
CREATE TABLE IF NOT EXISTS monitor.disk_io (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    device TEXT NOT NULL,
    read_count BIGINT,
    write_count BIGINT,
    read_bytes BIGINT,
    write_bytes BIGINT,
    read_time_ms BIGINT,
    write_time_ms BIGINT,
    busy_time_ms BIGINT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_disk_io_ts ON monitor.disk_io (ts DESC);
CREATE INDEX IF NOT EXISTS idx_disk_io_device_ts ON monitor.disk_io (device, ts DESC);

-- 5. Network IO (por interface)
CREATE TABLE IF NOT EXISTS monitor.net_io (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    iface TEXT NOT NULL,
    bytes_sent BIGINT,
    bytes_recv BIGINT,
    packets_sent BIGINT,
    packets_recv BIGINT,
    errin BIGINT,
    errout BIGINT,
    dropin BIGINT,
    dropout BIGINT,
    speed_mbps REAL,
    is_up BOOLEAN,
    mtu INT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_net_io_ts ON monitor.net_io (ts DESC);
CREATE INDEX IF NOT EXISTS idx_net_io_iface ON monitor.net_io (iface, ts DESC);

-- 6. Network addresses
CREATE TABLE IF NOT EXISTS monitor.net_addr (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    iface TEXT NOT NULL,
    family TEXT,
    address TEXT,
    netmask TEXT,
    broadcast TEXT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_net_addr_ts ON monitor.net_addr (ts DESC);

-- 7. GPU (NVIDIA)
CREATE TABLE IF NOT EXISTS monitor.gpu (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    gpu_index INT NOT NULL,
    name TEXT,
    uuid TEXT,
    driver_version TEXT,
    utilization_gpu_percent REAL,
    utilization_memory_percent REAL,
    utilization_encoder_percent REAL,
    utilization_decoder_percent REAL,
    memory_total_bytes BIGINT,
    memory_used_bytes BIGINT,
    memory_free_bytes BIGINT,
    temperature_gpu_c REAL,
    temperature_memory_c REAL,
    power_draw_w REAL,
    power_limit_w REAL,
    fan_speed_percent REAL,
    clock_graphics_mhz REAL,
    clock_memory_mhz REAL,
    clock_sm_mhz REAL,
    pcie_tx_bytes BIGINT,
    pcie_rx_bytes BIGINT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_gpu_ts ON monitor.gpu (ts DESC);
CREATE INDEX IF NOT EXISTS idx_gpu_index_ts ON monitor.gpu (gpu_index, ts DESC);

-- 8. Sensors (temperaturas, fans, voltages)
CREATE TABLE IF NOT EXISTS monitor.sensors (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    sensor_type TEXT NOT NULL, -- temperature, fan, voltage, power, etc
    name TEXT NOT NULL,
    label TEXT,
    value REAL,
    unit TEXT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_sensors_ts ON monitor.sensors (ts DESC);
CREATE INDEX IF NOT EXISTS idx_sensors_type ON monitor.sensors (sensor_type, ts DESC);
CREATE INDEX IF NOT EXISTS idx_sensors_type_name_ts ON monitor.sensors (sensor_type, name, ts DESC);

-- 9. Processes (top N)
CREATE TABLE IF NOT EXISTS monitor.processes (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    pid INT NOT NULL,
    ppid INT,
    name TEXT,
    exe TEXT,
    cmdline TEXT,
    username TEXT,
    status TEXT,
    cpu_percent REAL,
    memory_percent REAL,
    memory_rss_bytes BIGINT,
    memory_vms_bytes BIGINT,
    memory_shared_bytes BIGINT,
    num_threads INT,
    num_handles INT,
    num_fds INT,
    io_read_bytes BIGINT,
    io_write_bytes BIGINT,
    io_read_count BIGINT,
    io_write_count BIGINT,
    create_time TIMESTAMPTZ,
    cwd TEXT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_processes_ts ON monitor.processes (ts DESC);
CREATE INDEX IF NOT EXISTS idx_processes_pid ON monitor.processes (pid, ts DESC);
CREATE INDEX IF NOT EXISTS idx_processes_name ON monitor.processes (name, ts DESC);

-- 10. Connections (TCP/UDP)
CREATE TABLE IF NOT EXISTS monitor.connections (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    fd INT,
    family TEXT,
    type TEXT,
    laddr_ip TEXT,
    laddr_port INT,
    raddr_ip TEXT,
    raddr_port INT,
    status TEXT,
    pid INT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_connections_ts ON monitor.connections (ts DESC);

-- 11. Services (Windows)
CREATE TABLE IF NOT EXISTS monitor.services (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT,
    status TEXT,
    start_type TEXT,
    pid INT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_services_ts ON monitor.services (ts DESC);

-- 12. System info
CREATE TABLE IF NOT EXISTS monitor.system_info (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    boot_time TIMESTAMPTZ,
    uptime_seconds BIGINT,
    os_name TEXT,
    os_version TEXT,
    os_build TEXT,
    arch TEXT,
    manufacturer TEXT,
    model TEXT,
    total_ram_bytes BIGINT,
    cpu_name TEXT,
    cpu_cores_physical INT,
    cpu_cores_logical INT,
    users TEXT[],
    logged_users JSONB,
    battery_percent REAL,
    battery_secs_left INT,
    battery_power_plugged BOOLEAN,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_system_info_ts ON monitor.system_info (ts DESC);

-- 13. EventLog summary
CREATE TABLE IF NOT EXISTS monitor.eventlog (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    log_name TEXT NOT NULL, -- System, Application
    level TEXT NOT NULL, -- Error, Warning, Information
    event_id INT,
    provider TEXT,
    count INT,
    latest_message TEXT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_eventlog_ts ON monitor.eventlog (ts DESC);

-- 14. Host heartbeat / coleta meta
CREATE TABLE IF NOT EXISTS monitor.heartbeat (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    collector TEXT NOT NULL,
    duration_ms REAL,
    rows_inserted INT,
    success BOOLEAN,
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_heartbeat_ts ON monitor.heartbeat (ts DESC);

-- 15. Physical Disk inventory (snapshot)
CREATE TABLE IF NOT EXISTS monitor.physical_disk (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    device_id TEXT NOT NULL,
    friendly_name TEXT,
    model TEXT,
    serial_number TEXT,
    firmware_version TEXT,
    media_type TEXT, -- SSD, HDD, SCM
    bus_type TEXT, -- SATA, NVMe, USB, RAID
    health_status TEXT,
    operational_status TEXT,
    size_bytes BIGINT,
    allocated_size BIGINT,
    is_boot BOOLEAN,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_physical_disk_ts ON monitor.physical_disk (ts DESC);
CREATE INDEX IF NOT EXISTS idx_physical_disk_device ON monitor.physical_disk (device_id, ts DESC);

-- 16. Disk SMART (time-series)
CREATE TABLE IF NOT EXISTS monitor.disk_smart (
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    hostname TEXT NOT NULL,
    device TEXT NOT NULL, -- /dev/sda, /dev/sdd
    model TEXT,
    serial_number TEXT,
    firmware_version TEXT,
    device_type TEXT, -- ata, nvme, scsi
    protocol TEXT, -- ATA, NVMe, SCSI
    smart_passed BOOLEAN,
    temperature_c REAL, -- unificado
    power_on_hours BIGINT,
    power_cycle_count BIGINT,
    percentage_used REAL, -- NVMe wear 0-100
    available_spare REAL,
    available_spare_threshold REAL,
    media_errors BIGINT,
    reallocated_sectors BIGINT,
    pending_sectors BIGINT,
    host_reads BIGINT,
    host_writes BIGINT,
    data_units_read BIGINT, -- NVMe
    data_units_written BIGINT,
    total_lbas_read BIGINT, -- ATA
    total_lbas_written BIGINT,
    unsafe_shutdowns BIGINT,
    raw JSONB
);
CREATE INDEX IF NOT EXISTS idx_disk_smart_ts ON monitor.disk_smart (ts DESC);
CREATE INDEX IF NOT EXISTS idx_disk_smart_device ON monitor.disk_smart (device, ts DESC);

-- View útil: último status por host
CREATE OR REPLACE VIEW monitor.v_last_heartbeat AS
SELECT DISTINCT ON (hostname, collector) hostname, collector, ts, success, error
FROM monitor.heartbeat ORDER BY hostname, collector, ts DESC;

-- Opcional TimescaleDB: descomente se extensão instalada
-- CREATE EXTENSION IF NOT EXISTS timescaledb;
-- SELECT create_hypertable('monitor.cpu', 'ts', if_not_exists=>TRUE, chunk_time_interval=>INTERVAL '1 day');
-- SELECT create_hypertable('monitor.memory', 'ts', if_not_exists=>TRUE);
-- etc.

-- Retenção sugerida (ajuste conforme disco):
-- Para TimescaleDB: SELECT add_retention_policy('monitor.cpu', INTERVAL '90 days');
-- Para PG puro, rode periodicamente:
-- DELETE FROM monitor.processes WHERE ts < now() - INTERVAL '30 days';
-- DELETE FROM monitor.connections WHERE ts < now() - INTERVAL '7 days';
