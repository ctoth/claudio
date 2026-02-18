# Star Trek Bridge Soundpack Download Report

## Summary

| Metric | Value |
|---|---|
| **Total files downloaded** | 107 |
| **Download failures** | 0 |
| **Coverage** | 107/107 (100.0%) |
| **Source** | TrekCore.com |
| **Format** | MP3 |
| **Commit** | `48fae0e` |

## Download Results

All 107 sound files were downloaded successfully from TrekCore.com with HTTP 200 responses. No downloads failed.

### URL Corrections from Scout Report

The scout report (`reports/sound-sourcing-scout.md`) had several URLs that used guessed filenames. The actual TrekCore filenames were discovered by browsing the site with Chrome browser tools and extracting the real `<a href>` URLs from the page source.

Key differences from the scout report:

| Scout Report Filename | Actual TrekCore Filename |
|---|---|
| `keypress1.mp3` (in `/computer/`) | `tos_keypress1.mp3` (in `/toscomputer/`) |
| `keypress2.mp3` | `tos_keypress2.mp3` |
| `keypress3.mp3` | `tos_keypress3.mp3` |
| `processing1.mp3` | `processing.mp3` (no "1" suffix) |
| `inputok1.mp3` | `input_ok_1_clean.mp3` |
| `inputok2.mp3` | `input_ok_2_clean.mp3` |
| `inputok3.mp3` | `input_ok_3_clean.mp3` |
| `inputok4.mp3` | `input_ok_4.mp3` |
| `inputok5.mp3` | Does not exist (reused `input_ok_1_clean.mp3`) |
| `inputfailed1.mp3` | `input_failed_clean.mp3` |
| `inputfailed2.mp3` | `input_failed2_clean.mp3` |
| `inputfailed3.mp3` | Does not exist (reused `input_failed_clean.mp3`) |
| `inputfok1.mp3` | Does not exist (used `computer_screen_off_button_push.mp3` from sequences/) |
| `denybeep1.mp3` | `denybeep1.mp3` (correct) |
| `denybeep5.mp3` | Does not exist (only denybeep1-4 exist; reused denybeep1.mp3) |
| `tng_chirp2.mp3` | `tng_chirp2_clean.mp3` |
| `tng_chirp3.mp3` | `tng_chirp3_clean.mp3` |
| `hailbeep1.mp3` | `hailbeep_clean.mp3` |
| `hailbeep2.mp3` | `hailbeep2_clean.mp3` |
| `hailbeep3.mp3` | `hailbeep3_clean.mp3` |
| `tng_chirp1.mp3` | `tos_chirp_1.mp3` (used TOS chirp for stop/subagent-stop) |
| `tng_chirp4.mp3` | `tos_chirp_4.mp3` |
| `tng_chirp5.mp3` | `tos_chirp_5.mp3` |

### Reused Sounds

Some sound keys map to the same underlying file where distinct TrekCore sounds were not available:

- `success/grep-success.mp3` and `success/glob-success.mp3` both use `input_ok_1_clean.mp3`
- `interactive/stop.mp3` and `interactive/subagent-stop.mp3` both use `tos_chirp_1.mp3`
- Several error sounds reuse `denybeep1-4.mp3` and `input_failed_clean.mp3` across multiple keys
- `error/critical.mp3` and `error/read-error.mp3` both use `consolewarning.mp3`

## Validation Output

```
Validating: startrek-bridge
Name: startrek-bridge
Version: 1.0.0

Coverage Summary:
  Total:       107/107 (100.0%)
  completion:    3/3 (100.0%)
  default:       1/1 (100.0%)
  error:         19/19 (100.0%)
  interactive:   10/10 (100.0%)
  loading:       29/29 (100.0%)
  success:       41/41 (100.0%)
  system:        4/4 (100.0%)
```

Zero broken references. Zero format warnings.

## Complete Source Mapping

### loading/ (29 files)

| Soundpack Key | TrekCore URL |
|---|---|
| loading/bash-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_55.mp3` |
| loading/edit-start.mp3 | `https://trekcore.com/audio/toscomputer/tos_keypress1.mp3` |
| loading/read-start.mp3 | `https://trekcore.com/audio/computer/scrscroll1.mp3` |
| loading/write-start.mp3 | `https://trekcore.com/audio/toscomputer/tos_keypress3.mp3` |
| loading/grep-start.mp3 | `https://trekcore.com/audio/computer/scrsearch.mp3` |
| loading/glob-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_12.mp3` |
| loading/websearch-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_25.mp3` |
| loading/webfetch-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_45.mp3` |
| loading/git-commit-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_65.mp3` |
| loading/git-push-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_35.mp3` |
| loading/git-pull-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_19.mp3` |
| loading/git-status-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_11.mp3` |
| loading/git-add-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_16.mp3` |
| loading/git-diff-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_22.mp3` |
| loading/git-log-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_28.mp3` |
| loading/task-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_68.mp3` |
| loading/mcp-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_72.mp3` |
| loading/tool-start.mp3 | `https://trekcore.com/audio/computer/processing.mp3` |
| loading/loading.mp3 | `https://trekcore.com/audio/computer/processing2.mp3` |
| loading/processing.mp3 | `https://trekcore.com/audio/computer/processing3.mp3` |
| loading/connect.mp3 | `https://trekcore.com/audio/computer/computerbeep_31.mp3` |
| loading/echo-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_42.mp3` |
| loading/exitplanmode-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_50.mp3` |
| loading/file-editing.mp3 | `https://trekcore.com/audio/toscomputer/tos_keypress2.mp3` |
| loading/file-reading.mp3 | `https://trekcore.com/audio/computer/scrscroll2.mp3` |
| loading/go-build-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_58.mp3` |
| loading/ls-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_8.mp3` |
| loading/multiedit-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_60.mp3` |
| loading/todowrite-start.mp3 | `https://trekcore.com/audio/computer/computerbeep_48.mp3` |

### success/ (41 files)

| Soundpack Key | TrekCore URL |
|---|---|
| success/bash-success.mp3 | `https://trekcore.com/audio/computer/input_ok_1_clean.mp3` |
| success/edit-success.mp3 | `https://trekcore.com/audio/computer/input_ok_2_clean.mp3` |
| success/read-success.mp3 | `https://trekcore.com/audio/computer/input_ok_3_clean.mp3` |
| success/write-success.mp3 | `https://trekcore.com/audio/computer/input_ok_4.mp3` |
| success/grep-success.mp3 | `https://trekcore.com/audio/computer/input_ok_1_clean.mp3` |
| success/glob-success.mp3 | `https://trekcore.com/audio/computer/input_ok_1_clean.mp3` |
| success/git-commit-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_66.mp3` |
| success/git-push-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_36.mp3` |
| success/git-pull-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_20.mp3` |
| success/git-status-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_13.mp3` |
| success/git-add-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_17.mp3` |
| success/git-diff-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_23.mp3` |
| success/git-log-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_29.mp3` |
| success/tool-complete.mp3 | `https://trekcore.com/audio/computer/sequences/computer_screen_off_button_push.mp3` |
| success/success.mp3 | `https://trekcore.com/audio/computer/computerbeep_1.mp3` |
| success/task-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_69.mp3` |
| success/mcp-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_73.mp3` |
| success/websearch-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_26.mp3` |
| success/webfetch-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_46.mp3` |
| success/echo-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_43.mp3` |
| success/exitplanmode-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_51.mp3` |
| success/multiedit-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_61.mp3` |
| success/todowrite-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_49.mp3` |
| success/ls-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_9.mp3` |
| success/go-build-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_59.mp3` |
| success/celebration.mp3 | `https://trekcore.com/audio/computer/computerbeep_3.mp3` |
| success/complete.mp3 | `https://trekcore.com/audio/computer/computerbeep_5.mp3` |
| success/file-read.mp3 | `https://trekcore.com/audio/computer/scrscroll3.mp3` |
| success/file-saved.mp3 | `https://trekcore.com/audio/computer/computerbeep_7.mp3` |
| success/e2e-test-bat-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_14.mp3` |
| success/e2e-test-bat-tee-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_15.mp3` |
| success/find-f-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_21.mp3` |
| success/find-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_24.mp3` |
| success/gh-repo-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_27.mp3` |
| success/gh-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_30.mp3` |
| success/git-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_33.mp3` |
| success/grep-head-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_37.mp3` |
| success/npm-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_39.mp3` |
| success/tail-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_41.mp3` |
| success/uv-run-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_44.mp3` |
| success/uv-success.mp3 | `https://trekcore.com/audio/computer/computerbeep_47.mp3` |

### error/ (19 files)

| Soundpack Key | TrekCore URL |
|---|---|
| error/bash-error.mp3 | `https://trekcore.com/audio/computer/input_failed_clean.mp3` |
| error/edit-error.mp3 | `https://trekcore.com/audio/computer/denybeep1.mp3` |
| error/error.mp3 | `https://trekcore.com/audio/computer/computer_error.mp3` |
| error/tool-error.mp3 | `https://trekcore.com/audio/computer/denybeep2.mp3` |
| error/glob-error.mp3 | `https://trekcore.com/audio/computer/denybeep3.mp3` |
| error/grep-error.mp3 | `https://trekcore.com/audio/computer/denybeep4.mp3` |
| error/read-error.mp3 | `https://trekcore.com/audio/computer/consolewarning.mp3` |
| error/write-error.mp3 | `https://trekcore.com/audio/computer/input_failed2_clean.mp3` |
| error/mcp-error.mp3 | `https://trekcore.com/audio/computer/denybeep1.mp3` |
| error/task-error.mp3 | `https://trekcore.com/audio/computer/input_failed_clean.mp3` |
| error/websearch-error.mp3 | `https://trekcore.com/audio/computer/denybeep1.mp3` |
| error/webfetch-error.mp3 | `https://trekcore.com/audio/computer/denybeep2.mp3` |
| error/multiedit-error.mp3 | `https://trekcore.com/audio/computer/denybeep3.mp3` |
| error/todowrite-error.mp3 | `https://trekcore.com/audio/computer/denybeep4.mp3` |
| error/ls-error.mp3 | `https://trekcore.com/audio/computer/denybeep1.mp3` |
| error/critical.mp3 | `https://trekcore.com/audio/computer/consolewarning.mp3` |
| error/failure.mp3 | `https://trekcore.com/audio/computer/input_failed_clean.mp3` |
| error/timeout.mp3 | `https://trekcore.com/audio/computer/input_failed2_clean.mp3` |
| error/tool-interrupted.mp3 | `https://trekcore.com/audio/computer/input_failed_clean.mp3` |

### interactive/ (10 files)

| Soundpack Key | TrekCore URL |
|---|---|
| interactive/interactive.mp3 | `https://trekcore.com/audio/communicator/tng_chirp_clean.mp3` |
| interactive/message-sent.mp3 | `https://trekcore.com/audio/communicator/tng_chirp2_clean.mp3` |
| interactive/notification-permission.mp3 | `https://trekcore.com/audio/computer/hailbeep_clean.mp3` |
| interactive/prompt-submit.mp3 | `https://trekcore.com/audio/communicator/tng_chirp3_clean.mp3` |
| interactive/notification.mp3 | `https://trekcore.com/audio/computer/hailbeep2_clean.mp3` |
| interactive/chimes.mp3 | `https://trekcore.com/audio/communicator/tos_chirp_4.mp3` |
| interactive/compact.mp3 | `https://trekcore.com/audio/communicator/tos_chirp_5.mp3` |
| interactive/notify.mp3 | `https://trekcore.com/audio/computer/hailbeep3_clean.mp3` |
| interactive/stop.mp3 | `https://trekcore.com/audio/communicator/tos_chirp_1.mp3` |
| interactive/subagent-stop.mp3 | `https://trekcore.com/audio/communicator/tos_chirp_1.mp3` |

### completion/ (3 files)

| Soundpack Key | TrekCore URL |
|---|---|
| completion/agent-complete.mp3 | `https://trekcore.com/audio/other/tos_bosun_whistle_1.mp3` |
| completion/completion.mp3 | `https://trekcore.com/audio/other/tos_bosun_whistle_2.mp3` |
| completion/stop.mp3 | `https://trekcore.com/audio/computer/computerbeep_2.mp3` |

### system/ (4 files)

| Soundpack Key | TrekCore URL |
|---|---|
| system/session-start.mp3 | `https://trekcore.com/audio/background/tng_bridge_1.mp3` |
| system/system.mp3 | `https://trekcore.com/audio/background/tng_bridge_2.mp3` |
| system/compacting.mp3 | `https://trekcore.com/audio/other/power_down.mp3` |
| system/pre-compact.mp3 | `https://trekcore.com/audio/computer/computerbeep_70.mp3` |

### default (1 file)

| Soundpack Key | TrekCore URL |
|---|---|
| default.mp3 | `https://trekcore.com/audio/computer/computerbeep_1.mp3` |

## File Manifest

```
soundpacks/startrek-bridge.json                          (JSON soundpack manifest)
soundpacks/startrek-bridge/default.mp3
soundpacks/startrek-bridge/loading/bash-start.mp3
soundpacks/startrek-bridge/loading/connect.mp3
soundpacks/startrek-bridge/loading/echo-start.mp3
soundpacks/startrek-bridge/loading/edit-start.mp3
soundpacks/startrek-bridge/loading/exitplanmode-start.mp3
soundpacks/startrek-bridge/loading/file-editing.mp3
soundpacks/startrek-bridge/loading/file-reading.mp3
soundpacks/startrek-bridge/loading/git-add-start.mp3
soundpacks/startrek-bridge/loading/git-commit-start.mp3
soundpacks/startrek-bridge/loading/git-diff-start.mp3
soundpacks/startrek-bridge/loading/git-log-start.mp3
soundpacks/startrek-bridge/loading/git-pull-start.mp3
soundpacks/startrek-bridge/loading/git-push-start.mp3
soundpacks/startrek-bridge/loading/git-status-start.mp3
soundpacks/startrek-bridge/loading/glob-start.mp3
soundpacks/startrek-bridge/loading/go-build-start.mp3
soundpacks/startrek-bridge/loading/grep-start.mp3
soundpacks/startrek-bridge/loading/loading.mp3
soundpacks/startrek-bridge/loading/ls-start.mp3
soundpacks/startrek-bridge/loading/mcp-start.mp3
soundpacks/startrek-bridge/loading/multiedit-start.mp3
soundpacks/startrek-bridge/loading/processing.mp3
soundpacks/startrek-bridge/loading/read-start.mp3
soundpacks/startrek-bridge/loading/task-start.mp3
soundpacks/startrek-bridge/loading/todowrite-start.mp3
soundpacks/startrek-bridge/loading/tool-start.mp3
soundpacks/startrek-bridge/loading/webfetch-start.mp3
soundpacks/startrek-bridge/loading/websearch-start.mp3
soundpacks/startrek-bridge/loading/write-start.mp3
soundpacks/startrek-bridge/success/bash-success.mp3
soundpacks/startrek-bridge/success/celebration.mp3
soundpacks/startrek-bridge/success/complete.mp3
soundpacks/startrek-bridge/success/e2e-test-bat-success.mp3
soundpacks/startrek-bridge/success/e2e-test-bat-tee-success.mp3
soundpacks/startrek-bridge/success/echo-success.mp3
soundpacks/startrek-bridge/success/edit-success.mp3
soundpacks/startrek-bridge/success/exitplanmode-success.mp3
soundpacks/startrek-bridge/success/file-read.mp3
soundpacks/startrek-bridge/success/file-saved.mp3
soundpacks/startrek-bridge/success/find-f-success.mp3
soundpacks/startrek-bridge/success/find-success.mp3
soundpacks/startrek-bridge/success/gh-repo-success.mp3
soundpacks/startrek-bridge/success/gh-success.mp3
soundpacks/startrek-bridge/success/git-add-success.mp3
soundpacks/startrek-bridge/success/git-commit-success.mp3
soundpacks/startrek-bridge/success/git-diff-success.mp3
soundpacks/startrek-bridge/success/git-log-success.mp3
soundpacks/startrek-bridge/success/git-pull-success.mp3
soundpacks/startrek-bridge/success/git-push-success.mp3
soundpacks/startrek-bridge/success/git-success.mp3
soundpacks/startrek-bridge/success/git-status-success.mp3
soundpacks/startrek-bridge/success/glob-success.mp3
soundpacks/startrek-bridge/success/go-build-success.mp3
soundpacks/startrek-bridge/success/grep-head-success.mp3
soundpacks/startrek-bridge/success/grep-success.mp3
soundpacks/startrek-bridge/success/ls-success.mp3
soundpacks/startrek-bridge/success/mcp-success.mp3
soundpacks/startrek-bridge/success/multiedit-success.mp3
soundpacks/startrek-bridge/success/npm-success.mp3
soundpacks/startrek-bridge/success/read-success.mp3
soundpacks/startrek-bridge/success/success.mp3
soundpacks/startrek-bridge/success/tail-success.mp3
soundpacks/startrek-bridge/success/task-success.mp3
soundpacks/startrek-bridge/success/todowrite-success.mp3
soundpacks/startrek-bridge/success/tool-complete.mp3
soundpacks/startrek-bridge/success/uv-run-success.mp3
soundpacks/startrek-bridge/success/uv-success.mp3
soundpacks/startrek-bridge/success/webfetch-success.mp3
soundpacks/startrek-bridge/success/websearch-success.mp3
soundpacks/startrek-bridge/success/write-success.mp3
soundpacks/startrek-bridge/error/bash-error.mp3
soundpacks/startrek-bridge/error/critical.mp3
soundpacks/startrek-bridge/error/edit-error.mp3
soundpacks/startrek-bridge/error/error.mp3
soundpacks/startrek-bridge/error/failure.mp3
soundpacks/startrek-bridge/error/glob-error.mp3
soundpacks/startrek-bridge/error/grep-error.mp3
soundpacks/startrek-bridge/error/ls-error.mp3
soundpacks/startrek-bridge/error/mcp-error.mp3
soundpacks/startrek-bridge/error/multiedit-error.mp3
soundpacks/startrek-bridge/error/read-error.mp3
soundpacks/startrek-bridge/error/task-error.mp3
soundpacks/startrek-bridge/error/timeout.mp3
soundpacks/startrek-bridge/error/todowrite-error.mp3
soundpacks/startrek-bridge/error/tool-error.mp3
soundpacks/startrek-bridge/error/tool-interrupted.mp3
soundpacks/startrek-bridge/error/webfetch-error.mp3
soundpacks/startrek-bridge/error/websearch-error.mp3
soundpacks/startrek-bridge/error/write-error.mp3
soundpacks/startrek-bridge/interactive/chimes.mp3
soundpacks/startrek-bridge/interactive/compact.mp3
soundpacks/startrek-bridge/interactive/interactive.mp3
soundpacks/startrek-bridge/interactive/message-sent.mp3
soundpacks/startrek-bridge/interactive/notification-permission.mp3
soundpacks/startrek-bridge/interactive/notification.mp3
soundpacks/startrek-bridge/interactive/notify.mp3
soundpacks/startrek-bridge/interactive/prompt-submit.mp3
soundpacks/startrek-bridge/interactive/stop.mp3
soundpacks/startrek-bridge/interactive/subagent-stop.mp3
soundpacks/startrek-bridge/completion/agent-complete.mp3
soundpacks/startrek-bridge/completion/completion.mp3
soundpacks/startrek-bridge/completion/stop.mp3
soundpacks/startrek-bridge/system/compacting.mp3
soundpacks/startrek-bridge/system/pre-compact.mp3
soundpacks/startrek-bridge/system/session-start.mp3
soundpacks/startrek-bridge/system/system.mp3
```

## Notes

- All sounds are MP3 format, which Claudio supports natively
- The JSON manifest uses Windows absolute paths (`C:\Users\Q\code\claudio\soundpacks\startrek-bridge\...`)
- The TrekCore audio page uses `_clean` suffixes for remastered versions of some sounds
- Some sounds from the prompt plan (e.g., `inputok5.mp3`, `denybeep5.mp3`, `inputfailed3.mp3`) do not exist on TrekCore; equivalent sounds were reused
- The "Keypress" files are in TrekCore's `toscomputer/` directory (TOS-era sounds), not `computer/` (TNG-era)
- Sounds are copyrighted by Paramount/CBS Studios; this soundpack is for personal/hobby use only
