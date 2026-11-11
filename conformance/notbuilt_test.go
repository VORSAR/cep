package conformance_test

import (
	"testing"
)

// TestNotBuiltListIsLaw pins the loader of conformance/notbuilt.json (a law path): each fault is the exact error the
// design names (s1-1c-1-design.md §4.1), an unknown key is the decoder's own error, and the real file parses with
// its nine entries. Expected texts are the design's, written before the loader existed.
func TestNotBuiltListIsLaw(t *testing.T) {
	t.Parallel()
	bad := []struct {
		name string
		data string
		want string
	}{
		{
			name: "a part, not a row",
			data: `{"schema": 1, "note": "n", "entries": [{"file": "x.cep", "construct": "c", "sprint": "S1.1c-2"}]}`,
			want: `entries[0]: "S1.1c-2" names a part, not a row`,
		},
		{
			name: "a duplicate file",
			data: `{"schema": 1, "note": "n", "entries": [` +
				`{"file": "x.cep", "construct": "c", "sprint": "S4.1"}, ` +
				`{"file": "x.cep", "construct": "d", "sprint": "S7"}]}`,
			want: `entries[1]: duplicate file "x.cep"`,
		},
		{
			name: "schema 2",
			data: `{"schema": 2, "note": "n", "entries": []}`,
			want: `schema 2, want 1`,
		},
		{
			name: "an unknown key",
			data: `{"schema": 1, "note": "n", "comment": "c", "entries": []}`,
			want: `json: unknown field "comment"`,
		},
		{
			name: "an empty construct",
			data: `{"schema": 1, "note": "n", "entries": [{"file": "x.cep", "construct": "", "sprint": "S4.1"}]}`,
			want: `entries[0]: file, construct and sprint must be non-empty`,
		},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseNotBuilt([]byte(tc.data))
			if err == nil {
				t.Fatalf("parseNotBuilt: got %v and no error, want %q", got, tc.want)
			}
			if err.Error() != tc.want {
				t.Fatalf("parseNotBuilt: error %q, want %q", err.Error(), tc.want)
			}
		})
	}
	t.Run("the real file", func(t *testing.T) {
		t.Parallel()
		got := loadNotBuilt(t)
		if len(got) != 9 {
			t.Fatalf("%s: %d entries, want 9: %v", notBuiltPath, len(got), got)
		}
		for file, e := range got {
			if e.File != file {
				t.Errorf("%s: keyed %q, entry says %q", notBuiltPath, file, e.File)
			}
		}
	})
}

// TestSprintRowGrammar pins sprintRow to the plan check's grammar (s1-1c-1-plan-check.md §6, (1) and (2)), keeping
// the row forms: a part (-<n>) is never a row.
func TestSprintRowGrammar(t *testing.T) {
	t.Parallel()
	valid := []string{"S1.1a", "S2.2a", "S10", "S3.3", "S3", "S1.1c"}
	invalid := []string{"s3.3", "S03.3", "S3.03", "S3.3A", "S3.3ab", "S0", "S3.0", "", " S1.1c", "S1.1c ", "S1.1c-1"}
	for _, id := range valid {
		if !sprintRow.MatchString(id) {
			t.Errorf("sprintRow refuses %q, a row", id)
		}
	}
	for _, id := range invalid {
		if sprintRow.MatchString(id) {
			t.Errorf("sprintRow admits %q, not a row", id)
		}
	}
}
