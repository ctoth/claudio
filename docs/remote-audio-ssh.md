---
layout: default
title: "Remote Audio Over SSH"
description: "Run Claudio on a remote Linux box and hear it on your local Windows or macOS speakers by forwarding a PulseAudio socket over SSH."
---

# Remote Audio Over SSH

Claudio plays sound on the machine where the coding agent runs. When the agent
runs on a remote box over SSH, that machine usually has no sound card, so
playback fails and Claudio stays silent. The failure is recorded in the log
file, not shown in the agent.

The fix is to forward a PulseAudio socket over the SSH connection so the audio
is rendered on your local machine. This page uses the PulseAudio server that
WSLg already runs on Windows, which is the common case for this project. The
same forwarding works from any host that exposes a Pulse socket.

The remote box can play through the socket in two ways:

- **Oto, the default backend.** Oto has its own PulseAudio client written in
  Go. It honors `PULSE_SERVER`, sends audio over the socket without shared
  memory, and asks for a small buffer. It needs no packages on the remote box:
  do Part 1 and Part 2 steps 2 and 4, and skip the rest.
- **`paplay` through the `system_command` backend.** This is the path the rest
  of this page walks through, and the one to use if Oto cannot connect or you
  want `paplay`'s own options. It needs the libpulse client setup below.

Rough shape:

```text
remote box                          local machine (WSL)
  claudio -> Oto, or paplay ->      /tmp/pulse-fwd.sock
                                      | SSH RemoteForward
                                      v
                                    /mnt/wslg/PulseServer -> Windows speakers
```

## Part 1: The Listening Machine (WSL)

Do this once.

Confirm the WSLg Pulse server socket exists:

```bash
ls -l /mnt/wslg/PulseServer
```

If the socket is listed, the Windows side is ready and nothing needs
installing.

Add a `RemoteForward` line to each host entry in `~/.ssh/config`:

```text
Host mybox
    HostName mybox.lan
    User <username>
    RemoteForward /tmp/pulse-fwd.sock /mnt/wslg/PulseServer
```

Every SSH session to that host now carries the audio socket automatically.

## Part 2: The Remote Machine

Do this once per box.

### 1. Install The Pulse Client And A Player

```bash
sudo apt install libpulse0 pulseaudio-utils
```

`pulseaudio-utils` provides `paplay`, which is the first command Claudio's
`system_command` backend looks for. No PulseAudio daemon is needed on the
remote box.

### 2. Let sshd Replace Stale Sockets

Add to `/etc/ssh/sshd_config`:

```text
StreamLocalBindUnlink yes
```

Then restart sshd:

```bash
sudo systemctl restart ssh
```

Without this, reconnecting fails to forward the socket because the previous
session's socket file is still in `/tmp`.

### 3. Disable Shared Memory In The Pulse Client

SSH socket forwarding cannot pass file descriptors, so the default memfd
transport fails with `Expected 1 memfd fd to be received over pipe; got 0`.
Force everything over the socket:

```bash
mkdir -p ~/.config/pulse
echo "enable-shm = no" >> ~/.config/pulse/client.conf
```

This applies to libpulse clients such as `paplay`. Oto uses a Go PulseAudio
client instead; these libpulse client settings do not configure it.

### 4. Point Audio At The Forwarded Socket

Add to `~/.bashrc` (or your shell's rc file):

```bash
if [ -S /tmp/pulse-fwd.sock ]; then
    export PULSE_SERVER=unix:/tmp/pulse-fwd.sock
    export CLAUDIO_AUDIO_BACKEND=system_command
fi
```

The socket test keeps console logins and non-forwarded sessions from breaking.

`PULSE_SERVER` is what makes both Oto and `paplay` reach your speakers.
`CLAUDIO_AUDIO_BACKEND=system_command` selects `paplay`; leave that line out to
use Oto. On native Linux, `auto` picks Oto, and it prefers `system_command`
only under WSL.

To pin `paplay` in config instead of the environment:

```json
{
  "audio_backend": "system_command"
}
```

in `~/.config/claudio/config.json` on the remote box. `PULSE_SERVER` still has
to come from the environment, since Claudio has no config field for it.

### 5. Optional: ALSA-Only Programs

Only needed for programs that speak raw ALSA and will not use Pulse. Claudio
itself does not need this unless `aplay` is the only player on the box.

```bash
sudo apt install libasound2-plugins
```

Create `/etc/asound.conf`:

```text
pcm.!default pulse
ctl.!default pulse
```

## Part 3: Test

Log out of the box if you were on it, then reconnect so the forward and the rc
export take effect:

```bash
ssh mybox
ls -l /tmp/pulse-fwd.sock
echo $PULSE_SERVER
```

The socket should exist and the variable should be set. If you installed
`pulseaudio-utils`, test the plumbing before testing Claudio:

```bash
paplay /usr/share/sounds/alsa/Front_Center.wav
```

Then test Claudio end to end:

```bash
claudio status
echo '{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | claudio
```

`claudio status` should report `system_command` (or `oto` if you skipped the
override). The hook payload should produce a sound on your local speakers.

## Environment Inheritance

Claudio reads `PULSE_SERVER` from its own process environment. A hook process
inherits it from the agent, which inherits it from the shell that started the
agent. That chain works for the normal case: SSH in, shell rc runs, start the
agent in that shell.

It breaks when the agent is not a child of a fresh SSH login shell:

- **tmux or screen sessions started before you connected.** The old session
  holds the old (or missing) `PULSE_SERVER`. Re-export it in the pane, or use
  `tmux setenv -g PULSE_SERVER unix:/tmp/pulse-fwd.sock` and restart the
  agent.
- **Agents started by systemd or a supervisor.** These do not source your shell
  rc at all. Put `PULSE_SERVER` in the unit's `Environment=`.
- **`sudo` playback.** Root does not inherit `PULSE_SERVER`. Do not run
  Claudio under `sudo`.

To confirm what the hook process actually saw, use the log file rather than
guessing:

```bash
export CLAUDIO_LOG_LEVEL=debug
tail -F ~/.cache/claudio/logs/claudio.log
```

Claudio's stderr handler is fixed at ERROR level, so `CLAUDIO_LOG_LEVEL=debug`
changes the log file only.

## Playback Latency

This section applies to `paplay`. Oto requests a buffer of about 100 ms, so it
does not have this problem.

When a Pulse client does not request a buffer size, the server picks a default
of roughly two seconds. Over a tunnel that shows up as a long silent gap before
a short sound plays, which for Claudio means feedback that arrives after the
tool call it was describing.

Claudio's `system_command` backend execs `paplay` directly with a
`--volume=<n>` argument and the file path. It does not go through a shell, so a
`paplay` alias in your rc file has no effect on Claudio, and it does not pass
`--latency-msec`.

If you hear that delay, put a wrapper earlier in `PATH` than the real binary:

```bash
mkdir -p ~/bin
cat > ~/bin/paplay <<'EOF'
#!/bin/sh
exec /usr/bin/paplay --latency-msec=200 "$@"
EOF
chmod +x ~/bin/paplay
```

Ensure `~/bin` precedes `/usr/bin` in `PATH` for the shell that launches the
agent. Claudio resolves players with `exec.LookPath`, so it picks up the
wrapper. Verify the wrapper is what gets found:

```bash
command -v paplay
```

For non-Claudio programs routed through the ALSA Pulse plugin, exporting
`PULSE_LATENCY_MSEC=200` covers the same problem. It does not apply to
`paplay`.

## Troubleshooting

**`claudio status` shows `oto` but you set up `paplay`.** The
`CLAUDIO_AUDIO_BACKEND=system_command` export did not reach this shell. Check
that the socket existed when the rc file ran.

**`Expected 1 memfd fd` plus `Protocol error`.** Step 3 was skipped. Add
`enable-shm = no` to `~/.config/pulse/client.conf`.

**Reconnect works but audio does not, socket missing.** `StreamLocalBindUnlink
yes` is not set, or sshd was not restarted. Check `ls -l /tmp/pulse-fwd.sock`
after a fresh login.

**ALSA `cannot find card '0'` errors.** `PULSE_SERVER` is not set in the
session that started the agent, or the socket was never forwarded. See
[Environment Inheritance](#environment-inheritance).

**`Connection refused` under `sudo`.** Root does not inherit `PULSE_SERVER`.
Do not use `sudo` for playback.

**`Access denied` from the Pulse server.** The server wants cookie
authentication. WSLg normally does not, but if yours does, copy
`~/.config/pulse/cookie` from the WSL side to the same path on the remote box.
Both libpulse and Oto read that file; Oto also honors `PULSE_COOKIE`.

**`open(): No such file or directory` from `paplay`.** The WAV file is missing
on that box. That is a soundpack problem, not a connection problem; see
[Soundpacks](soundpacks).

**Long silent delay proportional to file length.** The client got the two
second server default buffer. See [Playback Latency](#playback-latency).

**`paplay` returns to the prompt well after audio ends.** Normal drain
confirmation delayed by the tunnel. Harmless. Claudio's hook returns
immediately and plays in a background process, so the agent does not wait.

**Claudio is silent but `paplay` works.** Confirm Claudio is not muted and the
volume is not zero:

```bash
claudio status
claudio unmute
```

## See Also

- [Configuration](configuration)
- [Troubleshooting](troubleshooting)
- [Soundpacks](soundpacks)
- [CLI Reference](cli-reference)
