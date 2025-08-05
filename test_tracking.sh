#!/bin/bash

export CLAUDIO_SOUND_TRACKING=true
export CLAUDIO_SOUND_TRACKING_DB=/tmp/test.db
export CLAUDIO_LOG_LEVEL=debug

echo '{"session_id":"test-session-123","transcript_path":"/test/transcript","cwd":"/test/path","hook_event_name":"PostToolUse","tool_name":"Edit","tool_response":{"stdout":"File updated successfully","stderr":"","interrupted":false}}' | ./claudio --silent

echo "Database file exists: $(ls -la /tmp/test.db 2>/dev/null || echo 'NO')"