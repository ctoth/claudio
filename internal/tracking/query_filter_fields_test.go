package tracking

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"

	"claudio.click/internal/hooks"
)

var jsonPathInClause = regexp.MustCompile(`'\$\.([A-Za-z_]+)'`)

// TestQueryFilter_ContentFiltersHitRecordedData sets each string filter on
// its own and checks every JSON path the WHERE clause reads is a key the
// recorder actually writes into hook_events.context. A filter on a key that
// is never written (the old $.SoundpackName) silently matches nothing.
func TestQueryFilter_ContentFiltersHitRecordedData(t *testing.T) {
	recorded, err := json.Marshal(&hooks.EventContext{})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(recorded, &keys); err != nil {
		t.Fatal(err)
	}

	values := map[string]string{"Category": "success"}
	typ := reflect.TypeFor[QueryFilter]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.String || f.Name == "OrderBy" || f.Name == "DatePreset" {
			continue
		}
		t.Run(f.Name, func(t *testing.T) {
			var q QueryFilter
			v := values[f.Name]
			if v == "" {
				v = "x"
			}
			reflect.ValueOf(&q).Elem().Field(i).SetString(v)

			clause, _, err := q.BuildWhereClause()
			if err != nil {
				t.Fatalf("BuildWhereClause: %v", err)
			}
			if clause == "" {
				t.Fatalf("filter %s set but produced no WHERE clause", f.Name)
			}
			for _, m := range jsonPathInClause.FindAllStringSubmatch(clause, -1) {
				if _, ok := keys[m[1]]; !ok {
					t.Errorf("filter %s queries $.%s, which the recorder never writes (clause %q)", f.Name, m[1], clause)
				}
			}
		})
	}
}
