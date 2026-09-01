from collectors.sensors import _clean_text


def test_hardware_text_replaces_postgresql_nul_bytes():
    assert _clean_text("sensor\x00name") == "sensor�name"
