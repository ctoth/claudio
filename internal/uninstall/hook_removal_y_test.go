package uninstall

import (
	"reflect"
	"testing"

	"claudio.click/internal/install"
)

// TestRemoveComplexClaudioHooksPreservesPreExistingEmptyItem regresses the
// drop-on-empty bug: a user's hand-edited {"matcher":"a","hooks":[]} entry
// would be silently deleted alongside the Claudio entry because the old code
// used "filteredHooks is empty" as a proxy for "we removed Claudio".
func TestRemoveComplexClaudioHooksPreservesPreExistingEmptyItem(t *testing.T) {
	preExistingEmpty := map[string]interface{}{
		"matcher": "user-block",
		"hooks":   []interface{}{},
	}
	claudioItem := map[string]interface{}{
		"matcher": "claudio-target",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "/usr/local/bin/claudio",
			},
		},
	}

	settings := &install.SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				claudioItem,
				preExistingEmpty,
			},
		},
	}

	removeComplexClaudioHooks(settings, []string{"PreToolUse"})

	hooksMap, ok := (*settings)["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("hooks section disappeared or wrong type: %T", (*settings)["hooks"])
	}

	pre, ok := hooksMap["PreToolUse"].([]interface{})
	if !ok {
		t.Fatalf("PreToolUse disappeared or wrong type: %T", hooksMap["PreToolUse"])
	}

	if len(pre) != 1 {
		t.Fatalf("expected exactly 1 surviving item (the pre-existing empty one); got %d: %v", len(pre), pre)
	}

	surviving, ok := pre[0].(map[string]interface{})
	if !ok {
		t.Fatalf("surviving item wrong type: %T", pre[0])
	}

	if surviving["matcher"] != "user-block" {
		t.Errorf("surviving item lost its matcher: got %v, want %q", surviving["matcher"], "user-block")
	}

	hooksSub, ok := surviving["hooks"].([]interface{})
	if !ok {
		t.Fatalf("surviving item's hooks field is wrong type (expected empty []interface{}, got %T)", surviving["hooks"])
	}
	if len(hooksSub) != 0 {
		t.Errorf("surviving item's hooks should still be empty; got %v", hooksSub)
	}
}

// TestRemoveComplexClaudioHooksItemsWithoutClaudioUnchanged: a three-item
// array where only one item has a Claudio command. The other two — a custom
// non-Claudio item and a pre-existing empty item — must both survive verbatim.
func TestRemoveComplexClaudioHooksItemsWithoutClaudioUnchanged(t *testing.T) {
	claudioItem := map[string]interface{}{
		"matcher": "claudio-target",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "/usr/local/bin/claudio",
			},
		},
	}
	userNonClaudio := map[string]interface{}{
		"matcher": "user-non-claudio",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "/usr/local/bin/lint",
			},
		},
	}
	userEmpty := map[string]interface{}{
		"matcher": "user-empty",
		"hooks":   []interface{}{},
	}

	settings := &install.SettingsMap{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				claudioItem,
				userNonClaudio,
				userEmpty,
			},
		},
	}

	removeComplexClaudioHooks(settings, []string{"PostToolUse"})

	hooksMap, _ := (*settings)["hooks"].(map[string]interface{})
	post, ok := hooksMap["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("PostToolUse disappeared or wrong type: %T", hooksMap["PostToolUse"])
	}

	if len(post) != 2 {
		t.Fatalf("expected exactly 2 surviving items; got %d: %v", len(post), post)
	}

	if !reflect.DeepEqual(post[0], userNonClaudio) {
		t.Errorf("user-non-claudio item not byte-identical to input.\nGot:  %v\nWant: %v", post[0], userNonClaudio)
	}
	if !reflect.DeepEqual(post[1], userEmpty) {
		t.Errorf("user-empty item not byte-identical to input.\nGot:  %v\nWant: %v", post[1], userEmpty)
	}
}
