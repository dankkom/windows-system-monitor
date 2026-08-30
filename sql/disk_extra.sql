-- Extensão para dados físicos e SMART de discos/SSD

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
