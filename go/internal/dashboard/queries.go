package dashboard

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var cpuSensorPatterns = []string{"%cpu%", "%core%", "%tctl%", "%ccd%"}

func bucketExpr(col string) string {
	if col == "" {
		col = "ts"
	}
	return "to_timestamp(floor(extract(epoch FROM " + col + ") / $1) * $1)"
}

// helper to scan time
func scanTime(v any) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	return time.Time{}
}

func QCPU(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, avg(cpu_total_percent), avg(freq_current_mhz) FROM monitor.cpu WHERE ts > now() - $2::interval GROUP BY bucket ORDER BY bucket"
	rows, err := pool.Query(ctx, sql, bucket, window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var cpu, freq *float64
		if err := rows.Scan(&ts, &cpu, &freq); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "cpu": cpu, "freq": freq})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QMemory(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, avg(used_percent), avg(used_bytes)/1073741824.0, avg(swap_used_percent) FROM monitor.memory WHERE ts > now() - $2::interval GROUP BY bucket ORDER BY bucket"
	rows, err := pool.Query(ctx, sql, bucket, window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var usedPct, usedGB, swap *float64
		if err := rows.Scan(&ts, &usedPct, &usedGB, &swap); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "used_percent": usedPct, "used_gb": usedGB, "swap": swap})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QGPU(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, avg(temperature_gpu_c), avg(utilization_gpu_percent), avg(power_draw_w), avg(memory_used_bytes)/1048576.0 FROM monitor.gpu WHERE ts > now() - $2::interval GROUP BY bucket ORDER BY bucket"
	rows, err := pool.Query(ctx, sql, bucket, window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var temp, util, power, vram *float64
		if err := rows.Scan(&ts, &temp, &util, &power, &vram); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "temp": temp, "util": util, "power": power, "vram": vram})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QCPUTemps(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	where := "(" + strings.Join([]string{"name ILIKE $2", "name ILIKE $3", "name ILIKE $4", "name ILIKE $5"}, " OR ") + ")"
	sql := "SELECT " + bucketSQL + " AS bucket, name, avg(value) FROM monitor.sensors WHERE sensor_type='temperature' AND " + where + " AND ts > now() - $6::interval AND value BETWEEN 0 AND 120 GROUP BY bucket, name ORDER BY bucket, name"
	rows, err := pool.Query(ctx, sql, bucket, cpuSensorPatterns[0], cpuSensorPatterns[1], cpuSensorPatterns[2], cpuSensorPatterns[3], window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var name string
		var val *float64
		if err := rows.Scan(&ts, &name, &val); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "name": name, "value": val})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QCPUTempsLatest(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	where := "(" + strings.Join([]string{"name ILIKE $1", "name ILIKE $2", "name ILIKE $3", "name ILIKE $4"}, " OR ") + ")"
	sql := "SELECT DISTINCT ON (name) name, value, unit, ts FROM monitor.sensors WHERE sensor_type='temperature' AND " + where + " AND value BETWEEN 10 AND 120 ORDER BY name, ts DESC"
	rows, err := pool.Query(ctx, sql, cpuSensorPatterns[0], cpuSensorPatterns[1], cpuSensorPatterns[2], cpuSensorPatterns[3])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name string
		var val *float64
		var unit *string
		var ts time.Time
		if err := rows.Scan(&name, &val, &unit, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": name, "value": val, "unit": unit, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QSensorsLatest(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT DISTINCT ON (sensor_type, name) name, sensor_type, value, unit, ts FROM monitor.sensors WHERE value IS NOT NULL ORDER BY sensor_type, name, ts DESC"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name, stype string
		var val *float64
		var unit *string
		var ts time.Time
		if err := rows.Scan(&name, &stype, &val, &unit, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": name, "type": stype, "value": val, "unit": unit, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QSensorsHistory(ctx context.Context, pool *pgxpool.Pool, window string, bucket int, sensorType string) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, "SELECT name FROM monitor.sensors WHERE sensor_type=$1 AND value IS NOT NULL GROUP BY name ORDER BY max(ts) DESC LIMIT 8", sensorType)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return []map[string]any{}, nil
	}
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, name, avg(value), max(unit) FROM monitor.sensors WHERE sensor_type=$2 AND name = ANY($3) AND value IS NOT NULL AND ts > now() - $4::interval GROUP BY bucket, name ORDER BY bucket, name"
	rows2, err := pool.Query(ctx, sql, bucket, sensorType, names, window)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	var out []map[string]any
	for rows2.Next() {
		var ts time.Time
		var name string
		var val *float64
		var unit *string
		if err := rows2.Scan(&ts, &name, &val, &unit); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "name": name, "value": val, "unit": unit})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows2.Err()
}

func QDiskUsage(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT DISTINCT ON (device) device, mountpoint, total_bytes, used_bytes, free_bytes, used_percent, ts FROM monitor.disk_usage ORDER BY device, ts DESC"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var device, mount *string
		var total, used, free *int64
		var pct *float64
		var ts time.Time
		if err := rows.Scan(&device, &mount, &total, &used, &free, &pct, &ts); err != nil {
			return nil, err
		}
		var freeGB *float64
		if free != nil {
			v := float64(*free) / 1073741824
			freeGB = &v
		}
		out = append(out, map[string]any{"device": derefStr(device), "mount": derefStr(mount), "total_bytes": total, "used_bytes": used, "free_bytes": free, "used_percent": pct, "free_gb": freeGB, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QDiskUsageHistory(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, device, max(mountpoint), avg(total_bytes), avg(used_bytes), avg(free_bytes), avg(used_percent) FROM monitor.disk_usage WHERE ts > now() - $2::interval GROUP BY bucket, device ORDER BY bucket, device"
	rows, err := pool.Query(ctx, sql, bucket, window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var device string
		var mount *string
		var total, used, free *float64
		var pct *float64
		if err := rows.Scan(&ts, &device, &mount, &total, &used, &free, &pct); err != nil {
			return nil, err
		}
		var ti, ui, fi *int64
		if total != nil {
			v := int64(*total)
			ti = &v
		}
		if used != nil {
			v := int64(*used)
			ui = &v
		}
		if free != nil {
			v := int64(*free)
			fi = &v
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "device": device, "mount": derefStr(mount), "total_bytes": ti, "used_bytes": ui, "free_bytes": fi, "used_percent": pct})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QPhysicalDisk(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT DISTINCT ON (device_id) device_id, friendly_name, model, media_type, bus_type, health_status, size_bytes/1073741824.0, ts FROM monitor.physical_disk ORDER BY device_id, ts DESC"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id string
		var friendly, model, media, bus, health *string
		var sizeGB *float64
		var ts time.Time
		if err := rows.Scan(&id, &friendly, &model, &media, &bus, &health, &sizeGB, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"device_id": id, "friendly_name": derefStr(friendly), "model": derefStr(model), "media_type": derefStr(media), "bus_type": derefStr(bus), "health": derefStr(health), "size_gb": sizeGB, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QDiskSmartLatest(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT DISTINCT ON (device) device, model, temperature_c, power_on_hours, power_cycle_count, percentage_used, available_spare, media_errors, reallocated_sectors, pending_sectors, smart_passed, ts FROM monitor.disk_smart ORDER BY device, ts DESC"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var device string
		var model *string
		var temp, wear, spare *float64
		var poh, pcycles, mediaErr, realloc, pending *int64
		var passed *bool
		var ts time.Time
		if err := rows.Scan(&device, &model, &temp, &poh, &pcycles, &wear, &spare, &mediaErr, &realloc, &pending, &passed, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"device": device, "model": derefStr(model), "temp": temp, "poh": poh, "pcycles": pcycles, "wear": wear, "spare": spare, "media_err": mediaErr, "realloc": realloc, "pending": pending, "passed": passed, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QDiskSmartHistory(ctx context.Context, pool *pgxpool.Pool, window string, bucket int) ([]map[string]any, error) {
	bucketSQL := bucketExpr("")
	sql := "SELECT " + bucketSQL + " AS bucket, device, avg(temperature_c) FROM monitor.disk_smart WHERE ts > now() - $2::interval AND temperature_c IS NOT NULL GROUP BY bucket, device ORDER BY bucket, device"
	rows, err := pool.Query(ctx, sql, bucket, window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var ts time.Time
		var device string
		var temp *float64
		if err := rows.Scan(&ts, &device, &temp); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ts": ts.Format(time.RFC3339Nano), "device": device, "temp": temp})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QDiskIO(ctx context.Context, pool *pgxpool.Pool, window string, bucket int, intervalDiskIO time.Duration) ([]map[string]any, error) {
	maxGap := int(intervalDiskIO.Seconds() * 3)
	bucketSQL := "to_timestamp(floor(extract(epoch FROM ts) / $2) * $2)"
	sql := "WITH raw AS (SELECT *, extract(epoch FROM ts-lag(ts) OVER w) AS dt, read_count-lag(read_count) OVER w AS drc, write_count-lag(write_count) OVER w AS dwc, read_bytes-lag(read_bytes) OVER w AS drb, write_bytes-lag(write_bytes) OVER w AS dwb, read_time_ms-lag(read_time_ms) OVER w AS drt, write_time_ms-lag(write_time_ms) OVER w AS dwt, busy_time_ms-lag(busy_time_ms) OVER w AS dbusy FROM monitor.disk_io WHERE ts > now() - $1::interval WINDOW w AS (PARTITION BY device ORDER BY ts)), valid AS (SELECT *, " + bucketSQL + " AS bucket FROM raw WHERE dt > 0 AND dt <= $3 AND drc >= 0 AND dwc >= 0 AND drb >= 0 AND dwb >= 0 AND drt >= 0 AND dwt >= 0 AND dbusy >= 0) SELECT bucket, device, sum(drb), sum(dwb), sum(drb)/nullif(sum(dt),0), sum(dwb)/nullif(sum(dt),0), sum(drc)/nullif(sum(dt),0), sum(dwc)/nullif(sum(dt),0), sum(drt)/nullif(sum(drc),0), sum(dwt)/nullif(sum(dwc),0), least(100.0, 100.0*sum(dbusy)/nullif(sum(dt)*1000,0)), count(*) FROM valid GROUP BY bucket, device ORDER BY bucket, device"
	rows, err := pool.Query(ctx, sql, window, bucket, maxGap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// pgx rows: bucket time, device string, 10 numeric cols + count
	type rec struct {
		ts     time.Time
		device string
		drb    *int64
		dwb    *int64
		rbps   *float64
		wbps   *float64
		riops  *float64
		wiops  *float64
		rlat   *float64
		wlat   *float64
		busy   *float64
		samples int64
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.ts, &r.device, &r.drb, &r.dwb, &r.rbps, &r.wbps, &r.riops, &r.wiops, &r.rlat, &r.wlat, &r.busy, &r.samples); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	cumulative := map[string][2]int64{}
	var out []map[string]any
	for _, r := range recs {
		cur := cumulative[r.device]
		if r.drb != nil {
			cur[0] += *r.drb
		}
		if r.dwb != nil {
			cur[1] += *r.dwb
		}
		cumulative[r.device] = cur
		out = append(out, map[string]any{
			"ts": r.ts.Format(time.RFC3339Nano), "device": r.device,
			"read": cur[0], "write": cur[1],
			"read_delta": r.drb, "write_delta": r.dwb,
			"read_bps": r.rbps, "write_bps": r.wbps,
			"read_iops": r.riops, "write_iops": r.wiops,
			"read_latency_ms": r.rlat, "write_latency_ms": r.wlat,
			"busy_percent": r.busy, "samples": r.samples, "valid": true,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func QNet(ctx context.Context, pool *pgxpool.Pool, window string, bucket int, intervalNet time.Duration) ([]map[string]any, error) {
	maxGap := int(intervalNet.Seconds() * 3)
	bucketSQL := "to_timestamp(floor(extract(epoch FROM ts) / $2) * $2)"
	sql := "WITH raw AS (SELECT *, extract(epoch FROM ts-lag(ts) OVER w) AS dt, bytes_recv-lag(bytes_recv) OVER w AS dbr, bytes_sent-lag(bytes_sent) OVER w AS dbs, packets_recv-lag(packets_recv) OVER w AS dpr, packets_sent-lag(packets_sent) OVER w AS dps, errin-lag(errin) OVER w AS dei, errout-lag(errout) OVER w AS deo, dropin-lag(dropin) OVER w AS ddi, dropout-lag(dropout) OVER w AS ddo FROM monitor.net_io WHERE ts > now() - $1::interval WINDOW w AS (PARTITION BY iface ORDER BY ts)), valid AS (SELECT *, " + bucketSQL + " AS bucket FROM raw WHERE dt > 0 AND dt <= $3 AND dbr >= 0 AND dbs >= 0 AND dpr >= 0 AND dps >= 0 AND dei >= 0 AND deo >= 0 AND ddi >= 0 AND ddo >= 0) SELECT bucket, iface, sum(dbr), sum(dbs), sum(dbr)/nullif(sum(dt),0), sum(dbs)/nullif(sum(dt),0), sum(dpr)/nullif(sum(dt),0), sum(dps)/nullif(sum(dt),0), sum(dei), sum(deo), sum(ddi), sum(ddo), max(speed_mbps), bool_or(is_up), max(mtu), count(*) FROM valid GROUP BY bucket, iface ORDER BY bucket, iface"
	rows, err := pool.Query(ctx, sql, window, bucket, maxGap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type rec struct {
		ts     time.Time
		iface  string
		dbr    *int64
		dbs    *int64
		rbps   *float64
		sbps   *float64
		rpps   *float64
		spps   *float64
		ei     *int64
		eo     *int64
		di     *int64
		do     *int64
		speed  *float64
		isUp   *bool
		mtu    *int64
		samples int64
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.ts, &r.iface, &r.dbr, &r.dbs, &r.rbps, &r.sbps, &r.rpps, &r.spps, &r.ei, &r.eo, &r.di, &r.do, &r.speed, &r.isUp, &r.mtu, &r.samples); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	cumulative := map[string][2]int64{}
	var out []map[string]any
	for _, r := range recs {
		cur := cumulative[r.iface]
		if r.dbr != nil {
			cur[0] += *r.dbr
		}
		if r.dbs != nil {
			cur[1] += *r.dbs
		}
		cumulative[r.iface] = cur
		var util *float64
		if r.speed != nil && *r.speed > 0 && r.rbps != nil && r.sbps != nil {
			speedBps := *r.speed * 1_000_000
			v := maxF(*r.rbps, *r.sbps) * 8 / speedBps * 100
			util = &v
		}
		out = append(out, map[string]any{
			"ts": r.ts.Format(time.RFC3339Nano), "iface": r.iface,
			"recv": cur[0], "sent": cur[1],
			"recv_delta": r.dbr, "sent_delta": r.dbs,
			"recv_bps": r.rbps, "sent_bps": r.sbps,
			"recv_pps": r.rpps, "sent_pps": r.spps,
			"errin": r.ei, "errout": r.eo, "dropin": r.di, "dropout": r.do,
			"speed_mbps": r.speed, "is_up": r.isUp, "mtu": r.mtu,
			"utilization_percent": util, "samples": r.samples, "valid": true,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func QNetLatest(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT DISTINCT ON (iface) iface, bytes_recv, bytes_sent, speed_mbps, is_up, mtu, ts FROM monitor.net_io ORDER BY iface, ts DESC"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var iface string
		var recv, sent *int64
		var speed *float64
		var isUp *bool
		var mtu *int64
		var ts time.Time
		if err := rows.Scan(&iface, &recv, &sent, &speed, &isUp, &mtu, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"iface": iface, "recv": recv, "sent": sent, "speed_mbps": speed, "is_up": isUp, "mtu": mtu, "ts": ts.Format(time.RFC3339Nano)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QProcesses(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT name, pid, cpu_percent, memory_percent, memory_rss_bytes/1048576.0, username FROM monitor.processes WHERE ts = (SELECT max(ts) FROM monitor.processes) ORDER BY cpu_percent DESC LIMIT 15"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var name *string
		var pid *int64
		var cpu, mem, rss *float64
		var user *string
		if err := rows.Scan(&name, &pid, &cpu, &mem, &rss, &user); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": derefStr(name), "pid": pid, "cpu": cpu, "mem": mem, "rss_mb": rss, "user": derefStr(user)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QSystem(ctx context.Context, pool *pgxpool.Pool) (map[string]any, error) {
	sql := "SELECT ts, hostname, uptime_seconds, cpu_name, os_build, total_ram_bytes/1073741824.0 FROM monitor.system_info ORDER BY ts DESC LIMIT 1"
	var ts time.Time
	var hostname *string
	var uptime *int64
	var cpuName, osBuild *string
	var ramGB *float64
	err := pool.QueryRow(ctx, sql).Scan(&ts, &hostname, &uptime, &cpuName, &osBuild, &ramGB)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ts": ts.Format(time.RFC3339Nano), "hostname": derefStr(hostname), "uptime": uptime, "cpu_name": derefStr(cpuName), "os_build": derefStr(osBuild), "ram_gb": ramGB}, nil
}

func QHeartbeat(ctx context.Context, pool *pgxpool.Pool) ([]map[string]any, error) {
	sql := "SELECT hostname, collector, ts, success, error FROM monitor.v_last_heartbeat ORDER BY collector"
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var host, collector string
		var ts time.Time
		var success *bool
		var errStr *string
		if err := rows.Scan(&host, &collector, &ts, &success, &errStr); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"host": host, "collector": collector, "ts": ts.Format(time.RFC3339Nano), "success": success, "error": derefStr(errStr)})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func QDBSize(ctx context.Context, pool *pgxpool.Pool) (map[string]any, error) {
	sql := "SELECT pg_size_pretty(pg_database_size(current_database())), (SELECT count(*) FROM monitor.cpu), (SELECT count(*) FROM monitor.sensors)"
	var size string
	var cpuRows, sensorRows int64
	if err := pool.QueryRow(ctx, sql).Scan(&size, &cpuRows, &sensorRows); err != nil {
		return nil, err
	}
	return map[string]any{"size": size, "cpu_rows": cpuRows, "sensor_rows": sensorRows}, nil
}

// helpers

func derefStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sortedKeys(m map[string][]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
