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
