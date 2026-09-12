"""Replay CLI audit findings entirely inside temporary XDG directories.

Usage: python reports/audit-2026-09-12/probe_cli.py CHECKOUT_EXE
The outside-directory marker remains inside this probe's temporary directory.
"""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import wave


def main():
    executable = str(Path(sys.argv[1]).resolve())
    results = {}
    with tempfile.TemporaryDirectory(prefix="claudio-cli-audit-") as temporary:
        root = Path(temporary)
        env = {k: v for k, v in os.environ.items() if not k.startswith("CLAUDIO_")}
        for key, value in {
            "XDG_CONFIG_HOME": root / "config",
            "XDG_CONFIG_DIRS": root / "system-config",
            "XDG_DATA_HOME": root / "data",
            "XDG_DATA_DIRS": root / "system-data",
            "XDG_CACHE_HOME": root / "cache",
            "APPDATA": root / "roaming",
            "LOCALAPPDATA": root / "local",
        }.items():
            value.mkdir(parents=True, exist_ok=True)
            env[key] = str(value)
        env["CLAUDIO_FILE_LOGGING"] = "false"
        env["CLAUDIO_SOUND_TRACKING"] = "false"
        config = root / "config" / "claudio" / "config.json"
        config.parent.mkdir(parents=True)
        initial_config = {
            "default_soundpack": "embedded:windows.json",
            "enabled": False,
            "volume": 0.23,
            "file_logging": {"enabled": False},
        }
        config.write_text(json.dumps(initial_config), encoding="utf-8")
        source = root / "source"
        source.mkdir()
        with wave.open(str(source / "tone.wav"), "wb") as audio:
            audio.setnchannels(1)
            audio.setsampwidth(2)
            audio.setframerate(8000)
            audio.writeframes(b"\x00\x00" * 80)

        def run(*args):
            process = subprocess.run(
                [executable, *map(str, args)], env=env, cwd=root,
                capture_output=True, timeout=15,
                creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
            )
            return {
                "exit_code": process.returncode,
                "stdout": process.stdout.decode(errors="replace"),
                "stderr": process.stderr.decode(errors="replace"),
            }

        manifest = source / "pack.json"
        manifest.write_text(json.dumps({
            "name": "audit-pack", "mappings": {"default.wav": "tone.wav"},
        }), encoding="utf-8")
        source_validation = run("soundpack", "validate", manifest)
        installation = run("soundpack", "install", manifest)
        installed = root / "data" / "claudio" / "soundpacks" / "audit-pack" / "soundpack.json"
        if not installed.exists():
            installed = root / "data" / "claudio" / "audit-pack.json"
        results["relative_assets"] = {
            "source_validation": source_validation,
            "installation": installation,
            "installed_validation": run("soundpack", "validate", installed),
            "installed_audio_exists": (installed.parent / "tone.wav").exists(),
        }

        escaped = root / "data" / "audit-escape.json"
        escaped.write_text("preserve-this-marker", encoding="utf-8")
        manifest.write_text(json.dumps({
            "name": "../audit-escape", "mappings": {"default.wav": "tone.wav"},
        }), encoding="utf-8")
        results["name_containment"] = {
            "installation": run("soundpack", "install", manifest),
            "outside_claudio_directory_overwritten": escaped.read_text() != "preserve-this-marker",
        }

        config.write_text("{recover me", encoding="utf-8")
        results["malformed_config"] = {
            "command": run("soundpack", "use", "windows"),
            "original_preserved": config.read_text() == "{recover me",
        }

        custom = root / "custom.json"
        custom.write_text(json.dumps(initial_config), encoding="utf-8")
        before_custom = custom.read_bytes()
        before_default = config.read_bytes()
        results["explicit_config"] = {
            "command": run("soundpack", "use", "linux", "--config", custom),
            "custom_changed": custom.read_bytes() != before_custom,
            "default_changed": config.read_bytes() != before_default,
        }
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
