# Where these fixtures came from

## claude_code_logs.json

A real OTLP logs export from Claude Code 2.1.296 on Windows, captured on
2026-10-10 by pointing one `claude -p` session at a local HTTP server:

```
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_LOGS_EXPORTER=otlp
OTEL_METRICS_EXPORTER=none
OTEL_EXPORTER_OTLP_PROTOCOL=http/json
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:14318
OTEL_LOGS_EXPORT_INTERVAL=1000
```

The session ran `echo hi` (succeeded) and `ls /nonexistent-dir-xyz` (failed)
through the Bash tool. Requests arrived as `POST /v1/logs` with
`Content-Type: application/json` and no `Content-Encoding`.

Changes made to the capture:

- Eight requests were merged into one, keeping six of the 83 records: one
  `hook_registered`, the `user_prompt`, one `tool_decision`, one
  `api_request` and both `tool_result` records. The session also sent
  `plugin_loaded`, `hook_execution_start`, `hook_execution_complete`,
  `mcp_server_connection`, `managed_settings_resolved` and
  `assistant_response` records, which are not kept here.
- `user.id`, `user.email`, `user.account_uuid`, `user.account_id` and
  `organization.id` were replaced with placeholders.
- A few attributes of the `api_request` record (cache token counts,
  request ids, `ttft_ms`, `cost_usd_micros`, `effort`) were dropped.

Everything else, including which values are strings (`success`,
`duration_ms`, `timeUnixNano`) and which are numbers (`event.sequence`), is
as it was sent. The prompt attributes read `<REDACTED>` in the original:
that is Claude Code's default.
