from pathlib import Path
from uuid import uuid4

import pytest

from spool import BatchSpool


def new_spool(max_bytes: int) -> BatchSpool:
    return BatchSpool(Path("logs") / f"pytest-{uuid4().hex}.sqlite3", max_bytes)


def test_replay_preserves_batch_order():
    spool = new_spool(100_000)
    spool.enqueue("monitor.cpu", ["value"], [(1,)])
    spool.enqueue("monitor.cpu", ["value"], [(2,)])
    received = []

    assert spool.replay(lambda table, columns, rows: received.append(rows[0][0]) or 1) == 2
    assert received == [1, 2]
    assert spool.status()["pending_batches"] == 0


def test_capacity_discards_oldest_batch():
    spool = new_spool(30)
    spool.enqueue("monitor.cpu", ["value"], [("first payload",)])
    spool.enqueue("monitor.cpu", ["value"], [("second payload",)])

    received = []
    spool.replay(lambda table, columns, rows: received.extend(rows) or len(rows))
    assert received == [("second payload",)]


def test_rejects_a_batch_larger_than_the_limit():
    spool = new_spool(10)
    with pytest.raises(ValueError, match="above buffer limit"):
        spool.enqueue("monitor.cpu", ["value"], [("payload larger than ten bytes",)])
