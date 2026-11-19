package plan

import (
	"reflect"
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/model"
)

// ukApp is S (k string, j int, ts timestamp) and O (k string), and one plain rule R from S into O keyed by keys
// with a 1 h ttl: a model the validator never saw, so a key it refuses (E-PART-004, E-NAME-001, E-PART-006) reaches
// Build.
func ukApp(keys ...model.PartitionKey) *model.App {
	src := model.Stream{Name: "S", Attrs: []model.Attr{{Name: "k", Type: event.TString}, {Name: "j", Type: event.TInt},
		{Name: "ts", Type: event.TTimestamp}}, Time: model.TimeClause{Kind: model.TimeAscending, Attr: "ts"}}
	out := model.Stream{Name: "O", Attrs: []model.Attr{{Name: "k", Type: event.TString}}}
	r := model.Rule{Name: "R", Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: "S"}},
		Select:    []model.SelectItem{{Expr: model.Expr{Kind: model.ExprAttr, Name: "k"}}},
		Partition: model.Partition{Keys: keys, TTL: model.DurationOrParam{Ms: 3_600_000}}, InsertInto: "O"}
	return &model.App{Streams: []model.Stream{src, out}, Rules: []model.Rule{r}}
}

// ukKey is `(names…) of stream`.
func ukKey(stream string, names ...string) model.PartitionKey {
	k := model.PartitionKey{Stream: stream}
	for _, n := range names {
		k.Attrs = append(k.Attrs, model.KeyAttr{Name: n})
	}
	return k
}

// TestBuildRefusesAnUncheckedPartition: a rule's partition keys as Build reads them (SPEC §4.4): only the source
// stream's key counts (a key of another stream leaves the rule key-less: KeyAttrs nil, the pseudo-key's one string
// type, ttl 0); a second key of the source is E-PART-004, an unknown key attribute E-NAME-001 and one named twice
// E-PART-006, each an error naming the defect, since Build is given only a checked model. The keyed rows are the
// controls: each key attribute's index and type in written order, and the ttl.
func TestBuildRefusesAnUncheckedPartition(t *testing.T) {
	t.Parallel()
	type key struct {
		attrs [][]int
		types []event.Type
		ttl   int64
	}
	keyless := key{nil, []event.Type{event.TString}, 0}
	for _, c := range []struct {
		name string
		keys []model.PartitionKey
		want key
		err  string // "" for a nil error
	}{
		{"no key", nil, keyless, ""},
		{"a key of another stream is skipped", []model.PartitionKey{ukKey("O", "k")}, keyless, ""},
		{"the source's key", []model.PartitionKey{ukKey("S", "j")}, key{[][]int{{1}}, []event.Type{event.TInt}, 3_600_000}, ""},
		{"another stream's key, then the source's", []model.PartitionKey{ukKey("O", "k"), ukKey("S", "j", "k")},
			key{[][]int{{1, 0}}, []event.Type{event.TInt, event.TString}, 3_600_000}, ""},
		{"keyed twice", []model.PartitionKey{ukKey("S", "k"), ukKey("S", "j")}, key{},
			"plan: rule R: stream S is keyed twice (E-PART-004; Build is given only a checked model)"},
		{"an unknown key attribute", []model.PartitionKey{ukKey("S", "k", "nope")}, key{},
			"plan: rule R: stream S: key attribute nope is unknown or named twice (a checked model has neither)"},
		{"a key attribute named twice", []model.PartitionKey{ukKey("S", "j", "k", "j")}, key{},
			"plan: rule R: stream S: key attribute j is unknown or named twice (a checked model has neither)"},
	} {
		p, err := Build(ukApp(c.keys...))
		if c.err != "" {
			if err == nil || err.Error() != c.err || p != nil {
				t.Errorf("%s: Build = %v, %v; want nil and %q", c.name, p, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: Build error %v; want none", c.name, err)
			continue
		}
		r := p.Rules[0]
		if got := (key{r.KeyAttrs, r.KeyTypes, r.TTL}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: KeyAttrs, KeyTypes, TTL = %v, %v, %d; want %v, %v, %d", c.name, got.attrs, got.types, got.ttl,
				c.want.attrs, c.want.types, c.want.ttl)
		}
	}
}
