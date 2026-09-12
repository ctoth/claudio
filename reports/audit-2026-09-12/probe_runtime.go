//go:build ignore

// Run explicitly with go run reports/audit-2026-09-12/probe_runtime.go.
// This probe uses only temporary files and private in-memory databases.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"claudio.click/internal/soundpack"
	"claudio.click/internal/tracking"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	root, err := os.MkdirTemp("", "claudio-runtime-audit-")
	check(err)
	defer os.RemoveAll(root)
	first := filepath.Join(root, "first.wav")
	second := filepath.Join(root, "second.wav")
	directory := filepath.Join(root, "directory.wav")
	check(os.WriteFile(first, []byte("exists"), 0600))
	check(os.WriteFile(second, []byte("exists"), 0600))
	check(os.Mkdir(directory, 0700))
	resolver := soundpack.NewSoundpackResolver(soundpack.NewJSONMapper("audit", map[string]string{
		"first.wav": first, "second.wav": second, "directory.wav": directory,
	}))
	var observations []map[string]any
	_, err = resolver.ResolveSoundWithFallback([]string{"first.wav", "second.wav"}, soundpack.WithObserver(func(path string, sequence int, exists bool) {
		observations = append(observations, map[string]any{"path": path, "reported_exists": exists})
	}))
	check(err)
	directoryResult, directoryErr := resolver.ResolveSound("directory.wav")

	db, err := tracking.NewDatabase(":memory:")
	check(err)
	defer db.Close()
	for _, tool := range []string{"Read", "Write"} {
		encoded, err := json.Marshal(map[string]any{"ToolName": tool, "Category": 1})
		check(err)
		_, err = db.Exec("INSERT INTO hook_events(timestamp,session_id,tool_name,selected_path,context) VALUES(1,'audit',?,'shared.wav',?)", tool, string(encoded))
		check(err)
	}
	usage, err := tracking.GetSoundUsage(db, tracking.QueryFilter{Tool: "Write"})
	check(err)

	// Holding the initialized connection forces the next query to open another.
	connection, err := db.Conn(context.Background())
	check(err)
	var count int
	queryContext, cancelQuery := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelQuery()
	secondConnectionErr := db.QueryRowContext(queryContext, "SELECT COUNT(*) FROM hook_events").Scan(&count)
	check(connection.Close())

	filter := tracking.QueryFilter{Category: "succes", DatePreset: "todai"}
	where, args := filter.BuildWhereClause()
	output := map[string]any{
		"observer_with_both_files_present": observations,
		"directory_accepted_as_sound": directoryErr == nil && directoryResult == directory,
		"usage_filtered_to_Write": usage,
		"second_memory_connection_error": fmt.Sprint(secondConnectionErr),
		"memory_max_open_connections": db.Stats().MaxOpenConnections,
		"invalid_filter_where": where,
		"invalid_filter_args": args,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	check(encoder.Encode(output))
}
