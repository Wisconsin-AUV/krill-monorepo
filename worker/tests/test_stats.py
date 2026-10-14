from pathlib import Path
from types import SimpleNamespace
from typing import Any

from krill_worker import stats
from krill_worker.heartbeat import Heartbeat
from krill_worker.models import Models


def write(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value + "\n")


def fake_drm(root: Path) -> Path:
    dev = root / "card1" / "device"
    write(dev / "gpu_busy_percent", "37")
    write(dev / "mem_info_vram_used", str(3 << 30))
    write(dev / "mem_info_vram_total", str(8 << 30))
    write(dev / "hwmon" / "hwmon3" / "temp1_input", "61000")
    write(dev / "hwmon" / "hwmon3" / "power1_average", "98000000")
    # A connector and an integrated card without busy stats are skipped.
    write(root / "card1-DP-1" / "device" / "gpu_busy_percent", "0")
    (root / "card0" / "device").mkdir(parents=True)
    return root


def test_amd_cards_reads_sysfs_units(tmp_path: Path) -> None:
    cards = stats.amd_cards(fake_drm(tmp_path))
    assert cards == [
        stats.AmdCard(
            busy_percent=37.0,
            vram_used=3 << 30,
            vram_total=8 << 30,
            temperature=61.0,
            power_watts=98.0,
        )
    ]


def unsupported(*_: Any) -> Any:
    raise ModuleNotFoundError("amdsmi")


def fake_torch(hip: str | None) -> Any:
    cuda = SimpleNamespace(
        is_available=lambda: True,
        device_count=lambda: 1,
        get_device_name=lambda i: "AMD Radeon RX 6600",
        mem_get_info=lambda i: (6 << 30, 8 << 30),
        utilization=unsupported,
        temperature=unsupported,
        power_draw=unsupported,
    )
    return SimpleNamespace(cuda=cuda, version=SimpleNamespace(hip=hip, cuda=None))


def test_rocm_without_amdsmi_falls_back_to_sysfs(tmp_path: Path, monkeypatch: Any) -> None:
    monkeypatch.setattr(stats, "DRM", fake_drm(tmp_path))
    [gpu] = stats.gpu_stats(fake_torch("6.4.43482-0f2d60242"))
    assert gpu.name == "AMD Radeon RX 6600"
    # torch's own memory numbers win over sysfs.
    assert (gpu.memory_used_bytes, gpu.memory_total_bytes) == (2 << 30, 8 << 30)
    assert gpu.utilization_percent == 37.0
    assert gpu.temperature_celsius == 61.0
    assert gpu.power_watts == 98.0
    assert stats.accelerator(fake_torch("6.4.43482-0f2d60242")) == "ROCm 6.4.43482"


def test_heartbeat_reports_the_running_task() -> None:
    models = Models(lambda m: SimpleNamespace(model_id=m))
    hb = Heartbeat(None, models, "CPU", lambda: [])  # type: ignore[arg-type]
    with hb.working(42):
        assert hb.request().task_id == 42
    req = hb.request()
    assert req.task_id == 0
    assert req.instance_id and req.host.memory_total_bytes > 0
