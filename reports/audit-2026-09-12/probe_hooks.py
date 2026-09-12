"""Compare hook startup and held-open stdin behavior without changing user settings.

Usage: python reports/audit-2026-09-12/probe_hooks.py INSTALLED_EXE CHECKOUT_EXE
The probe disables audio, file logging, and tracking with a temporary config.
Only processes started by this probe are terminated on timeout.
"""

import json
from pathlib import Path
import statistics
import subprocess
import sys
import tempfile
import time


def measure(args, payload, hold_open=False):
    start = time.perf_counter()
    proc = subprocess.Popen(
        args,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
    )
    timed_out = False
    try:
        if hold_open:
            proc.stdin.write(payload)
            proc.stdin.flush()
            proc.wait(timeout=3)
            proc.stdin.close()
            proc.stdin = None
            stdout, stderr = proc.communicate(timeout=3)
        else:
            stdout, stderr = proc.communicate(payload, timeout=15)
    except subprocess.TimeoutExpired:
        timed_out = True
        proc.kill()
        stdout, stderr = proc.communicate()
    return {
        "milliseconds": round((time.perf_counter() - start) * 1000, 1),
        "timed_out": timed_out,
        "exit_code": proc.returncode,
        "stdout": stdout.decode(errors="replace"),
        "stderr": stderr.decode(errors="replace"),
    }


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    payload = json.dumps({
        "session_id": "claudio-audit",
        "transcript_path": None,
        "cwd": str(Path.cwd()),
        "hook_event_name": "PreToolUse",
        "tool_name": "Bash",
        "tool_input": {"command": "git status --short"},
    }).encode()
    results = {}
    with tempfile.TemporaryDirectory(prefix="claudio-audit-") as temporary:
        config = Path(temporary) / "config.json"
        config.write_text(json.dumps({
            "default_soundpack": "embedded:windows.json",
            "enabled": False,
            "file_logging": {"enabled": False},
            "sound_tracking": {"enabled": False},
        }), encoding="utf-8")
        for label, executable in zip(("installed", "checkout"), sys.argv[1:]):
            args = [str(Path(executable).resolve()), "--silent", "--config", str(config)]
            samples = [measure(args, payload) for _ in range(5)]
            results[label] = {
                "executable": args[0],
                "normal_stdin": samples,
                "median_ms": statistics.median(s["milliseconds"] for s in samples),
                "held_open_stdin": measure(args, payload, hold_open=True),
            }
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
