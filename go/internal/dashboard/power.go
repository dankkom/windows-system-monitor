package dashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dankkom/windows-system-monitor/go/internal/config"
)

var windowSeconds = map[string]int{
	"15 minutes": 900, "1 hour": 3600, "6 hours": 21600, "24 hours": 86400,
	"7 days": 604800, "30 days": 2592000,
}

func powerSourcePriority(name string) int {
	norm := strings.ToLower(name)
	if strings.Contains(norm, "cpu package") {
		return 0
	}
	if strings.Contains(norm, "cpu platform") {
		return 1
	}
	parts := strings.Split(norm, ":")
	if len(parts) >= 2 {
		hw := strings.TrimSpace(parts[0])
		leaf := strings.TrimSpace(parts[len(parts)-1])
		if hw == "cpu" || hw == "processor" {
			if leaf == "package" || leaf == "cpu package" {
				return 0
			}
			if leaf == "platform" || leaf == "platform controller" {
				return 1
			}
		}
	}
	return 99
}

type powerPoint struct {
	TS           time.Time
	TSStr        string
	CPU          *float64
	GPUMeasured  *float64
	GPUEstimated *float64
	Measured     *float64
	Estimated    *float64
	CumMeasured  float64
	CumEstimated float64
}

func QPower(ctx context.Context, pool *pgxpool.Pool, cfg *config.Settings, window string, bucket int) (map[string]any, error) {
	bucketSQL := bucketExpr("")
	sensorSQL := "SELECT " + bucketSQL + " AS bucket, name, avg(value), max(unit), max(ts) FROM monitor.sensors WHERE sensor_type='power' AND value >= 0 AND ts > now() - $2::interval GROUP BY bucket, name ORDER BY bucket, name"
	gpuSQL := "SELECT " + bucketSQL + " AS bucket, avg(power_draw_w), avg(utilization_gpu_percent), max(ts) FROM monitor.gpu WHERE ts > now() - $2::interval GROUP BY bucket ORDER BY bucket"

	type sensorRow struct {
		bucket time.Time
		name   string
		val    *float64
		unit   *string
		ts     time.Time
	}
	var sensorRows []sensorRow
	rows, err := pool.Query(ctx, sensorSQL, bucket, window)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r sensorRow
		if err := rows.Scan(&r.bucket, &r.name, &r.val, &r.unit, &r.ts); err != nil {
			rows.Close()
			return nil, err
		}
		sensorRows = append(sensorRows, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	type gpuRow struct {
		bucket time.Time
		power  *float64
		util   *float64
		ts     time.Time
	}
	var gpuRowsList []gpuRow
	rows2, err := pool.Query(ctx, gpuSQL, bucket, window)
	if err != nil {
		return nil, err
	}
	for rows2.Next() {
		var r gpuRow
		if err := rows2.Scan(&r.bucket, &r.power, &r.util, &r.ts); err != nil {
			rows2.Close()
			return nil, err
		}
		gpuRowsList = append(gpuRowsList, r)
	}
	rows2.Close()
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, r := range sensorRows {
		if powerSourcePriority(r.name) < 99 {
			counts[r.name]++
		}
	}
	var cpuName *string
	bestPri := 100
	bestCount := -1
	for name, cnt := range counts {
		pri := powerSourcePriority(name)
		if pri < bestPri || (pri == bestPri && cnt > bestCount) {
			cn := name
			cpuName = &cn
			bestPri = pri
			bestCount = cnt
		}
	}

	byTS := map[time.Time]map[string]any{}
	sourceStats := map[string][]float64{}
	for _, r := range sensorRows {
		if r.val != nil {
			sourceStats[r.name] = append(sourceStats[r.name], *r.val)
		}
		if _, ok := byTS[r.bucket]; !ok {
			byTS[r.bucket] = map[string]any{"actual_ts": r.ts}
		}
		if m := byTS[r.bucket]; r.ts.After(m["actual_ts"].(time.Time)) {
			m["actual_ts"] = r.ts
		}
		if cpuName != nil && r.name == *cpuName && r.val != nil {
			byTS[r.bucket]["cpu_w"] = *r.val
		}
	}
	for _, r := range gpuRowsList {
		if _, ok := byTS[r.bucket]; !ok {
			byTS[r.bucket] = map[string]any{"actual_ts": r.ts}
		}
		if m := byTS[r.bucket]; r.ts.After(m["actual_ts"].(time.Time)) {
			m["actual_ts"] = r.ts
		}
		if r.power != nil {
			byTS[r.bucket]["gpu_measured_w"] = *r.power
		}
		if r.util != nil {
			byTS[r.bucket]["gpu_utilization"] = *r.util
		}
	}

	gpuModel := cfg.PowerGPUIdleW != nil && cfg.PowerGPUMaxW != nil
	var sorted []time.Time
	for k := range byTS {
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	var series []powerPoint
	var lastCPU *struct{ val float64; ts time.Time }
	var lastGPU *struct{ val float64; ts time.Time }
	var lastUtil *struct{ val float64; ts time.Time }
	intervalSensors := cfg.Intervals["sensors"]
	intervalGPU := cfg.Intervals["gpu"]
	if intervalSensors == 0 {
		intervalSensors = 15 * time.Second
	}
	if intervalGPU == 0 {
		intervalGPU = 10 * time.Second
	}
	for _, ts := range sorted {
		raw := byTS[ts]
		if v, ok := raw["cpu_w"].(float64); ok {
			lastCPU = &struct{ val float64; ts time.Time }{v, ts}
		}
		if v, ok := raw["gpu_measured_w"].(float64); ok {
			lastGPU = &struct{ val float64; ts time.Time }{v, ts}
		}
		if v, ok := raw["gpu_utilization"].(float64); ok {
			lastUtil = &struct{ val float64; ts time.Time }{v, ts}
		}
		var cpu *float64
		if lastCPU != nil && ts.Sub(lastCPU.ts) <= intervalSensors*3 {
			cpu = &lastCPU.val
		}
		var gpuMeasured *float64
		if lastGPU != nil && ts.Sub(lastGPU.ts) <= intervalGPU*3 {
			gpuMeasured = &lastGPU.val
		}
		var gpuUtil *float64
		if lastUtil != nil && ts.Sub(lastUtil.ts) <= intervalGPU*3 {
			gpuUtil = &lastUtil.val
		}
		var gpuEstimated *float64
		if gpuMeasured == nil && gpuModel && gpuUtil != nil {
			frac := *gpuUtil / 100
			if frac < 0 {
				frac = 0
			}
			if frac > 1 {
				frac = 1
			}
			v := *cfg.PowerGPUIdleW + (*cfg.PowerGPUMaxW-*cfg.PowerGPUIdleW)*frac
			gpuEstimated = &v
		}
		var measured *float64
		if cpu != nil || gpuMeasured != nil {
			sum := 0.0
			if cpu != nil {
				sum += *cpu
			}
			if gpuMeasured != nil {
				sum += *gpuMeasured
			}
			measured = &sum
		}
		var estimated *float64
		if cpu != nil {
			gpuPart := 0.0
			if gpuMeasured != nil {
				gpuPart = *gpuMeasured
			} else if gpuEstimated != nil {
				gpuPart = *gpuEstimated
			}
			v := (*cpu + gpuPart + cfg.PowerAuxBaselineW) / cfg.PowerPSUEfficiency
			estimated = &v
		}
		actualTS := raw["actual_ts"].(time.Time)
		series = append(series, powerPoint{TS: actualTS, TSStr: actualTS.Format(time.RFC3339Nano), CPU: cpu, GPUMeasured: gpuMeasured, GPUEstimated: gpuEstimated, Measured: measured, Estimated: estimated})
	}

	maxGap := bucket * 2
	if int(intervalGPU.Seconds()*3) > maxGap {
		maxGap = int(intervalGPU.Seconds() * 3)
	}
	if int(intervalSensors.Seconds()*3) > maxGap {
		maxGap = int(intervalSensors.Seconds() * 3)
	}
	loc, _ := time.LoadLocation(cfg.DashboardTimezone)
	if loc == nil {
		loc = time.UTC
	}
	measuredWh, measuredCovered, measuredDaily := integratePower(series, "measured", maxGap, loc)
	estimatedWh, estimatedCovered, daily := integratePower(series, "estimated", maxGap, loc)
	ws := windowSeconds[window]
	coverage := 0.0
	if ws > 0 {
		coverage = minF(100, estimatedCovered/float64(ws)*100)
	}
	measuredCoverage := 0.0
	if ws > 0 {
		measuredCoverage = minF(100, measuredCovered/float64(ws)*100)
	}
	var sources []map[string]any
	for name, vals := range sourceStats {
		latest := vals[len(vals)-1]
		minV := vals[0]
		maxV := vals[0]
		sum := 0.0
		for _, v := range vals {
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
			sum += v
		}
		avg := sum / float64(len(vals))
		included := cpuName != nil && name == *cpuName
		reason := "excluído para evitar sobreposição"
		if included {
			reason = "CPU canônica"
		}
		sources = append(sources, map[string]any{"name": name, "unit": "W", "latest": latest, "minimum": minV, "average": avg, "maximum": maxV, "samples": len(vals), "included": included, "reason": reason})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i]["name"].(string) < sources[j]["name"].(string) })
	gpuMeasuredAvail := false
	for _, p := range series {
		if p.GPUMeasured != nil {
			gpuMeasuredAvail = true
			break
		}
	}
	var gpuVals []float64
	for _, r := range gpuRowsList {
		if r.power != nil {
			gpuVals = append(gpuVals, *r.power)
		}
	}
	var gpuLatest, gpuMin, gpuAvg, gpuMax *float64
	if len(gpuVals) > 0 {
		l := gpuVals[len(gpuVals)-1]
		gpuLatest = &l
		mi := gpuVals[0]
		ma := gpuVals[0]
		s := 0.0
		for _, v := range gpuVals {
			if v < mi {
				mi = v
			}
			if v > ma {
				ma = v
			}
			s += v
		}
		gpuMin = &mi
		gpuMax = &ma
		a := s / float64(len(gpuVals))
		gpuAvg = &a
	}
	gpuIncluded := gpuMeasuredAvail || gpuModel
	gpuReason := "potência indisponível; modelo desativado"
	if gpuMeasuredAvail {
		gpuReason = "potência nativa"
	} else if gpuModel {
		gpuReason = "modelo por utilização"
	}
	sources = append(sources, map[string]any{"name": "GPU (NVML)", "unit": "W", "latest": gpuLatest, "minimum": gpuMin, "average": gpuAvg, "maximum": gpuMax, "samples": len(gpuVals), "included": gpuIncluded, "reason": gpuReason})

	defaults := cfg.PowerAuxBaselineW == 30.0 && cfg.PowerPSUEfficiency == 0.90
	quality := "estimated_calibrated"
	if !gpuMeasuredAvail && !gpuModel {
		quality = "partial"
	} else if defaults {
		quality = "estimated_default"
	}
	periods := periodTotals(daily, measuredDaily)
	var peak *float64
	var avgEst *float64
	if estimatedCovered > 0 {
		v := estimatedWh * 3600 / estimatedCovered
		avgEst = &v
	}
	for _, p := range series {
		if p.Estimated != nil {
			if peak == nil || *p.Estimated > *peak {
				v := *p.Estimated
				peak = &v
			}
		}
	}
	var seriesOut []map[string]any
	for _, p := range series {
		seriesOut = append(seriesOut, map[string]any{
			"ts": p.TSStr, "cpu_w": p.CPU, "gpu_measured_w": p.GPUMeasured, "gpu_estimated_w": p.GPUEstimated,
			"measured_w": p.Measured, "estimated_w": p.Estimated,
			"cumulative_measured_wh": p.CumMeasured, "cumulative_estimated_wh": p.CumEstimated,
		})
	}
	if seriesOut == nil {
		seriesOut = []map[string]any{}
	}
	var cpuSource any
	if cpuName != nil {
		cpuSource = *cpuName
	}
	return map[string]any{
		"meta": map[string]any{
			"window": window, "bucket_seconds": bucket, "timezone": cfg.DashboardTimezone,
			"quality": quality, "cpu_source": cpuSource, "gpu_power_available": gpuMeasuredAvail,
			"gpu_model_enabled": gpuModel, "aux_baseline_w": cfg.PowerAuxBaselineW,
			"psu_efficiency": cfg.PowerPSUEfficiency,
			"coverage_percent": coverage,
		},
		"summary": map[string]any{
			"measured_wh": measuredWh, "estimated_wh": estimatedWh,
			"average_estimated_w": avgEst,
			"peak_estimated_w": peak,
			"measured_coverage_percent": measuredCoverage,
		},
		"series": seriesOut, "periods": periods, "sources": sources,
	}, nil
}

func integratePower(series []powerPoint, field string, maxGap int, loc *time.Location) (float64, float64, map[string]float64) {
	cumulative := 0.0
	covered := 0.0
	daily := map[string]float64{}
	var prev *struct {
		TS    time.Time
		Value *float64
	}
	for i := range series {
		var curVal *float64
		if field == "measured" {
			curVal = series[i].Measured
		} else {
			curVal = series[i].Estimated
		}
		if field == "measured" {
			series[i].CumMeasured = cumulative
		} else {
			series[i].CumEstimated = cumulative
		}
		if prev != nil && curVal != nil && prev.Value != nil {
			dt := series[i].TS.Sub(prev.TS).Seconds()
			if dt > 0 && dt <= float64(maxGap) {
				wh := (*prev.Value + *curVal) / 2 * dt / 3600
				cumulative += wh
				covered += dt
				mid := prev.TS.Add(time.Duration(dt/2*1e9) * time.Nanosecond)
				day := mid.In(loc).Format("2006-01-02")
				daily[day] += wh
				if field == "measured" {
					series[i].CumMeasured = cumulative
				} else {
					series[i].CumEstimated = cumulative
				}
			}
		}
		prev = &struct {
			TS    time.Time
			Value *float64
		}{series[i].TS, curVal}
	}
	return cumulative, covered, daily
}

func periodTotals(estimated, measured map[string]float64) map[string]any {
	weekly := map[string][2]float64{}
	monthly := map[string][2]float64{}
	keys := map[string]bool{}
	for k := range estimated {
		keys[k] = true
	}
	for k := range measured {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var daily []map[string]any
	for _, day := range sorted {
		est := estimated[day]
		meas := measured[day]
		d, _ := time.Parse("2006-01-02", day)
		isoYear, isoWeek := d.ISOWeek()
		wk := fmt.Sprintf("%d-W%02d", isoYear, isoWeek)
		w := weekly[wk]
		w[0] += meas
		w[1] += est
		weekly[wk] = w
		mk := d.Format("2006-01")
		m := monthly[mk]
		m[0] += meas
		m[1] += est
		monthly[mk] = m
		daily = append(daily, map[string]any{"period": day, "measured_wh": meas, "estimated_wh": est})
	}
	if daily == nil {
		daily = []map[string]any{}
	}
	var weeklyOut []map[string]any
	for k, v := range weekly {
		weeklyOut = append(weeklyOut, map[string]any{"period": k, "measured_wh": v[0], "estimated_wh": v[1]})
	}
	sort.Slice(weeklyOut, func(i, j int) bool { return weeklyOut[i]["period"].(string) < weeklyOut[j]["period"].(string) })
	var monthlyOut []map[string]any
	for k, v := range monthly {
		monthlyOut = append(monthlyOut, map[string]any{"period": k, "measured_wh": v[0], "estimated_wh": v[1]})
	}
	sort.Slice(monthlyOut, func(i, j int) bool { return monthlyOut[i]["period"].(string) < monthlyOut[j]["period"].(string) })
	if weeklyOut == nil {
		weeklyOut = []map[string]any{}
	}
	if monthlyOut == nil {
		monthlyOut = []map[string]any{}
	}
	return map[string]any{"daily": daily, "weekly": weeklyOut, "monthly": monthlyOut}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
