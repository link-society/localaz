package monitorserver

import (
	"strings"
	"testing"

	"localaz.dev/internal/stores/monitorstore"
)

// newTestStore returns a store seeded with a small "Logs" table covering
// string, number and boolean columns.
func newTestStore(t *testing.T) *monitorstore.Store {
	t.Helper()
	s := monitorstore.New()
	s.Ingest("Custom-Logs", []monitorstore.Row{
		{"Level": "Error", "Code": float64(500), "Active": true, "Msg": "boom"},
		{"Level": "Warn", "Code": float64(400), "Active": false, "Msg": "careful"},
		{"Level": "Info", "Code": float64(200), "Active": true, "Msg": "ok"},
		{"Level": "Info", "Code": float64(204), "Active": false, "Msg": "no content"},
	})
	return s
}

func TestEvalKQLValid(t *testing.T) {
	store := newTestStore(t)

	cases := []struct {
		name     string
		query    string
		wantRows int
	}{
		{"bare table", "Logs", 4},

		// where with each operator
		{"where eq string", `Logs | where Level == "Error"`, 1},
		{"where ne string", `Logs | where Level != "Info"`, 2},
		{"where lt number", "Logs | where Code < 300", 2},
		{"where le number", "Logs | where Code <= 204", 2},
		{"where gt number", "Logs | where Code > 300", 2},
		{"where ge number", "Logs | where Code >= 400", 2},
		{"where eq bool", "Logs | where Active == true", 2},
		{"where ne bool", "Logs | where Active != true", 2},

		// and / or
		{"where and", `Logs | where Level == "Info" and Active == true`, 1},
		{"where or", `Logs | where Level == "Error" or Level == "Warn"`, 2},

		// where with quoted value containing the operator-ish chars and keywords
		{"where quoted value with keyword", `Logs | where Msg == "no content"`, 1},

		// take / limit
		{"take", "Logs | take 2", 2},
		{"limit alias", "Logs | limit 1", 1},
		{"take more than rows", "Logs | take 100", 4},

		// project does not change row count
		{"project", "Logs | project Level, Code", 4},

		// sort by (does not change count)
		{"sort by asc", "Logs | sort by Code asc", 4},
		{"order by alias", "Logs | order by Code desc", 4},

		// chained pipeline
		{"where then take", `Logs | where Active == true | take 1`, 1},
		{"where then project then take", `Logs | where Code >= 200 | project Level | take 3`, 3},

		// count collapses to one row
		{"count", "Logs | count", 1},
		{"where then count", `Logs | where Level == "Info" | count`, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl, err := evalKQL(store, tc.query)
			if err != nil {
				t.Fatalf("evalKQL(%q) error: %v", tc.query, err)
			}
			if len(tbl.rows) != tc.wantRows {
				t.Fatalf("evalKQL(%q) rows=%d, want %d", tc.query, len(tbl.rows), tc.wantRows)
			}
		})
	}
}

func TestEvalKQLCountValue(t *testing.T) {
	store := newTestStore(t)
	tbl, err := evalKQL(store, `Logs | where Level == "Info" | count`)
	if err != nil {
		t.Fatalf("evalKQL count error: %v", err)
	}
	if len(tbl.rows) != 1 {
		t.Fatalf("count rows=%d, want 1", len(tbl.rows))
	}
	if got := tbl.rows[0]["Count"]; got != float64(2) {
		t.Fatalf("Count=%v, want 2", got)
	}
}

func TestEvalKQLSortOrder(t *testing.T) {
	store := newTestStore(t)
	tbl, err := evalKQL(store, "Logs | sort by Code asc | project Code")
	if err != nil {
		t.Fatalf("evalKQL sort error: %v", err)
	}
	var got []float64
	for _, r := range tbl.rows {
		got = append(got, r["Code"].(float64))
	}
	want := []float64{200, 204, 400, 500}
	if len(got) != len(want) {
		t.Fatalf("sorted rows=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted[%d]=%v, want %v (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestEvalKQLMalformed(t *testing.T) {
	store := newTestStore(t)

	cases := []struct {
		name  string
		query string
	}{
		{"empty query", ""},
		{"unknown table", "DoesNotExist"},
		{"unsupported operator", "Logs | summarize count()"},
		{"where missing operator", "Logs | where Level"},
		{"where missing rhs", "Logs | where Level =="},
		{"where missing lhs", `Logs | where == "Error"`},
		{"where unsupported literal", "Logs | where Level == bareword"},
		{"take non-numeric", "Logs | take abc"},
		{"take negative", "Logs | take -1"},
		{"project no columns", "Logs | project"},
		{"sort missing by", "Logs | sort Code"},
		{"sort bad direction", "Logs | sort by Code sideways"},
		{"empty where", "Logs | where"},
		{"parentheses unsupported", `Logs | where (Level == "Error")`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl, err := func() (tbl resultTable, err error) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("evalKQL(%q) panicked: %v", tc.query, r)
					}
				}()
				return evalKQL(store, tc.query)
			}()
			if err == nil {
				t.Fatalf("evalKQL(%q) = nil error, want error (rows=%d)", tc.query, len(tbl.rows))
			}
		})
	}
}

// TestEvalKQLAdversarial throws large and pathological inputs at the parser to
// ensure they fail gracefully (an error or an empty/full result) without
// panicking.
func TestEvalKQLAdversarial(t *testing.T) {
	store := newTestStore(t)

	inputs := []string{
		strings.Repeat("|", 10000),
		strings.Repeat("Logs | where Level == \"x\" ", 1000),
		"Logs | " + strings.Repeat("where Code > 0 and ", 500) + "Code > 0",
		`Logs | where Msg == "` + strings.Repeat("a", 100000) + `"`,
		strings.Repeat("(", 10000),
		"Logs |||| where",
	}

	for i, q := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("adversarial input #%d panicked: %v", i, r)
				}
			}()
			// We don't assert on the result; only that it returns without panic.
			_, _ = evalKQL(store, q)
		}()
	}
}
