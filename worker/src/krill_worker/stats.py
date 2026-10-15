"""Host and GPU stats for the admin panel's compute page."""

import logging
import platform
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import psutil
from krill.v1 import system_pb2

log = logging.getLogger(__name__)

DRM = Path("/sys/class/drm")


def host_stats() -> system_pb2.HostStats:
    mem = psutil.virtual_memory()
    disk = psutil.disk_usage("/")
    return system_pb2.HostStats(
        cpu_percent=psutil.cpu_percent(interval=None),
        cpu_count=psutil.cpu_count() or 0,
        memory_used_bytes=mem.total - mem.available,
        memory_total_bytes=mem.total,
        disk_used_bytes=disk.used,
        # Leaves out the blocks the filesystem reserves, like the API does.
        disk_total_bytes=disk.used + disk.free,
        platform=f"{platform.system()} {platform.release()}",
    )


@dataclass
class AmdCard:
    busy_percent: float | None
    vram_used: int | None
    vram_total: int | None
    temperature: float | None
    power_watts: float | None


def _read_int(path: Path) -> int | None:
    try:
        return int(path.read_text().strip())
    except (OSError, ValueError):
        return None


def amd_cards(root: Path | None = None) -> list[AmdCard]:
    """Read AMD GPUs from sysfs, since ROCm images rarely ship amdsmi."""
    cards = []
    for card in sorted((root or DRM).glob("card*"), key=lambda p: p.name):
        # Skips connectors like card0-DP-1.
        if not re.fullmatch(r"card\d+", card.name):
            continue
        dev = card / "device"
        busy = _read_int(dev / "gpu_busy_percent")
        if busy is None:
            continue
        temp = power = None
        for hwmon in sorted((dev / "hwmon").glob("hwmon*")):
            if (t := _read_int(hwmon / "temp1_input")) is not None:
                temp = t / 1000
            # power1_average is in microwatts; newer kernels only have input.
            for name in ("power1_average", "power1_input"):
                if (p := _read_int(hwmon / name)) is not None:
                    power = p / 1_000_000
                    break
        cards.append(
            AmdCard(
                busy_percent=float(busy),
                vram_used=_read_int(dev / "mem_info_vram_used"),
                vram_total=_read_int(dev / "mem_info_vram_total"),
                temperature=temp,
                power_watts=power,
            )
        )
    return cards


def _try(fn: Any, *args: Any) -> Any:
    try:
        return fn(*args)
    except Exception:
        return None


def gpu_stats(torch: Any) -> list[system_pb2.GpuStats]:
    """Stats for each GPU torch sees, filling gaps from sysfs on AMD."""
    if not torch.cuda.is_available():
        return []
    count = torch.cuda.device_count()
    amd = amd_cards() if torch.version.hip else []
    # Only trust the sysfs order when it lines up with torch's.
    if len(amd) != count:
        amd = []
    out = []
    for i in range(count):
        g = system_pb2.GpuStats(index=i, name=torch.cuda.get_device_name(i))
        if (mem := _try(torch.cuda.mem_get_info, i)) is not None:
            free, total = mem
            g.memory_used_bytes, g.memory_total_bytes = total - free, total
        util = _try(torch.cuda.utilization, i)
        temp = _try(torch.cuda.temperature, i)
        power = _try(torch.cuda.power_draw, i)
        if power is not None:
            power /= 1000
        if amd:
            card = amd[i]
            util = card.busy_percent if util is None else util
            temp = card.temperature if temp is None else temp
            power = card.power_watts if power is None else power
            if not g.memory_total_bytes and card.vram_total:
                g.memory_used_bytes, g.memory_total_bytes = card.vram_used or 0, card.vram_total
        if util is not None:
            g.utilization_percent = float(util)
        if temp is not None:
            g.temperature_celsius = float(temp)
        if power is not None:
            g.power_watts = float(power)
        out.append(g)
    return out


def accelerator(torch: Any) -> str:
    if torch.version.hip:
        return f"ROCm {torch.version.hip.split('-')[0]}"
    if torch.version.cuda and torch.cuda.is_available():
        return f"CUDA {torch.version.cuda}"
    return "CPU"
