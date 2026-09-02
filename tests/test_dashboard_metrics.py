from datetime import datetime, timedelta, timezone
from zoneinfo import ZoneInfo

from dashboard.queries_light import _integrate_power, _period_totals, _power_source_priority


def test_power_integration_breaks_on_gap_and_uses_trapezoid():
    start = datetime(2026, 1, 1, tzinfo=timezone.utc)
    points = [
        {"_ts": start, "power": 10.0},
        {"_ts": start + timedelta(hours=1), "power": 20.0},
        {"_ts": start + timedelta(hours=3), "power": 100.0},
    ]
    total, covered, daily = _integrate_power(points, "power", 3600, ZoneInfo("UTC"))
    assert total == 15.0
    assert covered == 3600
    assert daily == {"2026-01-01": 15.0}
    assert points[-1]["cumulative_power_wh"] == 15.0


def test_cpu_package_has_priority_and_periods_are_summed():
    assert _power_source_priority("CPU Package") < _power_source_priority("CPU Platform")
    assert _power_source_priority("CPU Package") < _power_source_priority("CPU Cores")
    periods = _period_totals({"2026-01-01": 10.0, "2026-01-02": 5.0})
    assert periods["weekly"] == [{"period": "2026-W01", "measured_wh": 0.0, "estimated_wh": 15.0}]
    assert periods["monthly"] == [{"period": "2026-01", "measured_wh": 0.0, "estimated_wh": 15.0}]


def test_power_source_priority_recognizes_lhm_composite_cpu_names():
    # LibreHardwareMonitor no Ryzen 7 5700X nomeia o sensor como
    # '<hw_type>:<hw_name>:<sensor>' -- o pacote da CPU fica fora do literal
    # 'CPU Package' e deve ainda assim ser escolhido como fonte canônica.
    assert _power_source_priority("Cpu:AMD Ryzen 7 5700X:Package") == 0
    assert _power_source_priority("CPU:Intel Core i7-9700K:CPU Package") < 99
    assert _power_source_priority("Processor:Generic CPU:Platform") == 1
    # núcleos individuais e GPU não devem ser tratados como fonte da CPU
    assert _power_source_priority("Cpu:AMD Ryzen 7 5700X:Core #1 (SMU)") == 99
    assert _power_source_priority("GpuNvidia:NVIDIA GeForce RTX 4060 Ti:GPU Package") == 99
