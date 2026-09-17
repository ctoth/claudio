#!/usr/bin/env bash
set -euo pipefail

temp_parent="${TMPDIR:-/tmp}"
temp_parent="$(cd -- "$temp_parent" && pwd -P)"
test_root="$(mktemp -d "$temp_parent/claudio-tracking.XXXXXX")"
test_root="$(cd -- "$test_root" && pwd -P)"

case "$test_root" in
  "$temp_parent"/claudio-tracking.*) ;;
  *)
    echo "refusing unsafe temporary path: $test_root" >&2
    exit 1
    ;;
esac

cleanup() {
  case "$test_root" in
    "$temp_parent"/claudio-tracking.*) rm -rf -- "$test_root" ;;
    *) echo "refusing unsafe cleanup path: $test_root" >&2 ;;
  esac
}
trap cleanup EXIT

binary="$test_root/claudio"
database="$test_root/cache/claudio/tracking-test.db"
mkdir -p "$(dirname "$database")" "$test_root/config" "$test_root/data"

cache_env="$test_root/cache"
config_env="$test_root/config"
data_env="$test_root/data"
database_env="$database"
if [[ "$(go env GOOS)" == "windows" ]]; then
  cache_env="$(cygpath -w "$cache_env")"
  config_env="$(cygpath -w "$config_env")"
  data_env="$(cygpath -w "$data_env")"
  database_env="$(cygpath -w "$database_env")"
fi

go build -o "$binary" ./cmd/claudio

payload='{"session_id":"test-session-123","transcript_path":"/test/transcript","cwd":"/test/path","hook_event_name":"PostToolUse","tool_name":"Edit","tool_response":{"stdout":"File updated successfully","stderr":"","interrupted":false}}'

printf '%s\n' "$payload" | \
  LOCALAPPDATA="$cache_env" \
  XDG_CACHE_HOME="$cache_env" \
  XDG_CONFIG_HOME="$config_env" \
  XDG_DATA_HOME="$data_env" \
  CLAUDIO_FILE_LOGGING=false \
  CLAUDIO_SOUND_TRACKING=true \
  CLAUDIO_SOUND_TRACKING_DB="$database_env" \
  "$binary" --silent

if [[ ! -s "$database" ]]; then
  echo "tracking database was not created" >&2
  exit 1
fi

echo "tracking smoke test passed: isolated database created"
