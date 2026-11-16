package validate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vorsar/cep/internal/model"
)

// Pieces of the operand cases: a pattern tail, a timed stream with and without idleness, and a given timestamp.
const (
	patTail = " within 5 min after match skip past last late events drop select "
	clocked = " time ts watermark bounded 1 min idle after 1 min;\n"
	given   = "timestamp '2026-01-01T10:00:00Z'"
)

// expectRule is a rule over from with the expect sections sections and a then over Out that matches its select.
func expectRule(from, sections string) string {
	return "rule R intent 'x' from " + from + " expect " + sections + " then Out [ {k: 'a', ts: " + given +
		"} ] select k insert into Out;\n"
}

// TestEachConditionOperandDecides gives each operand of each compound condition in the functions lane V splits a
// case where it alone decides the result (design s1-1c-6 §5 §3.2): each row's findings are the codes, in check's
// sorted order, and msg (when set) is in one of the messages.
func TestEachConditionOperandDecides(t *testing.T) {
	t.Parallel()
	negative := Options{Overrides: map[string]model.Literal{"w": {Kind: model.LitDuration, Ms: -60_000}}}
	cases := []struct {
		name, rules, want, msg string
		not                    string // a text no message may hold
		opts                   Options
	}{
		// partition: the part of a grouped key that is an object (bounds.go's E-PART-008 at the part).
		{name: "partition_object_part", rules: "stream Q (k string, o object, ts timestamp)" + clocked +
			"rule P intent 'x' partition by (k, o) of Q ttl 1 hour from every a = Q" + patTail + "a.k insert into Out;\n",
			want: "E-PART-008", msg: "part 2 of the partition key: o of Q is an object"},
		// zeroDurations: skipDur, a negative value, a written zero, an absent slot.
		{name: "zero_slot_from_undeclared_param", rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" +
			" within ${nope} after match skip past last late events drop select a.k insert into Out;\n", want: "E-PARAM-001"},
		{name: "zero_negative_override", rules: "param w duration = 5 min;\nrule P intent 'x' partition by k of S ttl 1 hour" +
			" from every a = S within ${w} after match skip past last late events drop select a.k insert into Out;\n",
			want: "E-DUR-003", msg: "negative (-60000 ms)", opts: negative},
		{name: "zero_written", rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S within 0 ms" +
			" after match skip past last late events drop select a.k insert into Out;\n", want: "E-DUR-003", msg: "is zero"},
		{name: "zero_absent_and_positive", rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" +
			patTail + "a.k insert into Out;\n"},
		// ttl: no partition, a ttl from an undeclared param, a ttl below a bound, a bound from an undeclared param.
		{name: "ttl_no_partition_with_rate", rules: "rule P intent 'x' from S select k output first every 5 min insert into Out;\n"},
		{name: "ttl_from_undeclared_param", rules: "rule P intent 'x' partition by k of S ttl ${nope} from every a = S" +
			patTail + "a.k insert into Out;\n", want: "E-PARAM-001"},
		{name: "ttl_below_within", rules: "rule P intent 'x' partition by k of S ttl 1 min from every a = S" +
			patTail + "a.k insert into Out;\n", want: "E-TTL-002"},
		{name: "ttl_bound_from_undeclared_param", rules: "rule P intent 'x' partition by k of S ttl 1 min from every a = S" +
			" within ${nope} after match skip past last late events drop select a.k insert into Out;\n", want: "E-PARAM-001"},
		// clauses: batch over a stream with no @source, and over a punctuating one.
		{name: "batch_without_source", rules: "rule P intent 'x' partition by k of S ttl 1 hour from S window batch()" +
			" emit on batch end late events drop select k insert into Out;\n"},
		{name: "batch_over_punctuating_source", rules: "@source(type = 'stdin') stream Q (k string, ts timestamp)" + clocked +
			"rule P intent 'x' partition by k of Q ttl 1 hour from Q window batch() emit on batch end late events drop" +
			" select k insert into Out;\n"},
		// annotations: no type item, an unknown type (keys unchecked), a known type with an unknown key.
		{name: "source_without_type", rules: "@source(path = '/x') stream Q (k string, ts timestamp)" + clocked + plain},
		{name: "source_unknown_type", rules: "@source(type = 'nope', pth = '/x') stream Q (k string, ts timestamp)" +
			clocked + plain, want: "E-NAME-001", msg: "unknown @source type 'nope'"},
		{name: "source_unknown_key", rules: "@source(type = 'http', pth = '/x') stream Q (k string, ts timestamp)" +
			clocked + plain, want: "E-NAME-001", msg: "did you mean path?"},
		// givenRecord: text_invisibles in a table and in a stream; optional attributes given null or left out.
		{name: "given_table_invisibles_column", rules: "table TI (k string, text_invisibles bool) primary key (k);\n" +
			expectRule("S[k not in TI]", "given table TI [ {k: 'a', text_invisibles: true} ] given S [ {k: 'a', n: 1, ts: "+
				given+"} ]")},
		{name: "given_table_invisibles_left_out", rules: "table TI (k string, text_invisibles bool) primary key (k);\n" +
			expectRule("S[k not in TI]", "given table TI [ {k: 'a'} ] given S [ {k: 'a', n: 1, ts: "+given+"} ]"),
			want: "E-EXP-002", msg: "TI.text_invisibles is required; the block leaves it out"},
		{name: "given_stream_invisibles_field", rules: "stream SI (k string, text_invisibles bool, ts timestamp)" + clocked +
			expectRule("SI", "given SI [ {k: 'a', text_invisibles: true, ts: "+given+"} ]"),
			want: "E-EXP-002", msg: "SI.text_invisibles is computed by the engine"},
		{name: "given_optional_null", rules: expectRule("S", "given S [ {k: 'a', n: 1, note: null, ts: "+given+"} ]")},
		{name: "given_optional_time_null", rules: "stream SO (k string, ts timestamp optional)" + clocked +
			expectRule("SO", "given SO [ {k: 'a', ts: null} ]"), want: "E-TIME-003 E-EXP-002",
			msg: "SO.ts is required; the block gives it null"},
		{name: "given_optional_time_left_out", rules: "stream SO (k string, ts timestamp optional)" + clocked +
			expectRule("SO", "given SO [ {k: 'a'} ]"), want: "E-TIME-003 E-EXP-002",
			msg: "SO.ts is required; the block leaves it out"},
		// before: an unknown and a one-column table; count() in a window select; chain() in a pattern select.
		{name: "in_unknown_table", rules: "rule P intent 'x' from S[k in Nope] select k insert into Out;\n",
			want: "E-NAME-001", msg: "unknown table Nope"},
		{name: "in_one_column_table", rules: "rule P intent 'x' from S[k in T] select k insert into Out;\n"},
		{name: "text_unknown_table", rules: "rule P intent 'x' from S[k has_any_ci(Nope)] select k insert into Out;\n",
			want: "E-NAME-001", msg: "unknown table Nope"},
		{name: "count_in_window_select", rules: "stream W (k string, n int, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from S window batch() emit on batch end late events drop" +
			" select k, count() as n insert into W;\n" + plain},
		{name: "chain_in_pattern_select", rules: "stream E (k string, e object, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from every a = S" + patTail + "a.k, chain(a) as e insert into E;\n" +
			plain},
		// build: a bare unknown name in a plain rule; a bare name no capture has in a pattern select.
		{name: "bare_unknown_in_plain", rules: "rule P intent 'x' from S select kk as k insert into Out;\n",
			want: "E-NAME-001", msg: "did you mean k?"},
		{name: "bare_name_a_capture_has", rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" +
			patTail + "k insert into Out;\n", want: "E-NAME-001",
			msg: "unknown attribute k; a pattern select names a capture's: did you mean a.k?"},
		{name: "bare_name_in_a_step_filter", rules: "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = S" +
			" -> b = R[n > 1]" + patTail + "a.k insert into Out;\n", want: "E-NAME-001", msg: "unknown attribute n",
			not: "a pattern select names"},
		{name: "bare_name_no_capture_has", rules: "rule P intent 'x' partition by k of S ttl 1 hour from every a = S" +
			patTail + "zz as k insert into Out;\n", want: "E-NAME-001", msg: "unknown attribute zz"},
		// nulls: is null over a non-attribute; != and < outside an absence; < inside one; a required attribute.
		{name: "is_null_over_coalesce", rules: "rule P intent 'x' from S[coalesce(note, 'x') is null] select k insert into Out;\n"},
		{name: "is_null_over_a_call_named_like_an_attribute", rules: "stream Q (k string, abs int optional, ts timestamp)" +
			clocked + "rule P intent 'x' from Q[abs(abs) is null or abs != 1] select k insert into Out;\n", want: "E-NULL-002"},
		{name: "text_operator_inside_absence", rules: "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = R" +
			" and not S[note has_ci 'x'] for 1 min" + patTail + "a.k insert into Out;\n"},
		{name: "not_equal_outside_absence", rules: "rule P intent 'x' from S[note != 'x'] select k insert into Out;\n",
			want: "E-NULL-002"},
		{name: "less_outside_absence", rules: "rule P intent 'x' from S[note < 'x'] select k insert into Out;\n"},
		{name: "less_inside_absence", rules: "rule P intent 'x' partition by k of S, k of R ttl 1 hour from every a = R" +
			" and not S[note < 'x'] for 1 min" + patTail + "a.k insert into Out;\n", want: "E-NULL-001"},
		{name: "required_attribute_not_equal", rules: "rule P intent 'x' from S[k != 'x'] select k insert into Out;\n"},
		// selects: an aliased attribute; an optional target; aggregating and non-aggregating windows; the ts check's
		// four operands; the sink check's four operands.
		{name: "aliased_attribute", rules: "stream O2 (k string, m int, ts timestamp);\n" +
			"rule P intent 'x' from S select k, n as m insert into O2;\n" + plain},
		{name: "unaliased_call", rules: "rule P intent 'x' from S select k, abs(n) insert into Out;\n", want: "E-SEL-001",
			msg: "select item 2 binds no attribute of Out"},
		{name: "optional_target_attribute", rules: "stream O2 (k string, note string optional, ts timestamp);\n" +
			"rule P intent 'x' from S select k, note insert into O2;\n" + plain},
		{name: "required_target_attribute", rules: "stream O2 (k string, note string, ts timestamp);\n" +
			"rule P intent 'x' from S select k, note insert into O2;\n" + plain, want: "E-NULL-003"},
		{name: "aggregating_window_per_event", rules: "stream W (k string, m int, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from S window batch() emit on batch end late events drop" +
			" select k, n as m insert into W;\n" + plain, want: "E-SEL-002"},
		{name: "sort_window_per_event", rules: "stream W (k string, n int, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from S window sort(3, n, desc) emit on change late events drop" +
			" select k, n insert into W;\n" + plain},
		{name: "target_without_ts", rules: "stream N (k string);\nrule P intent 'x' from S select k insert into N;\n" + plain},
		{name: "timed_target_with_string_ts", rules: "stream TT (k string, ts string, at timestamp)" +
			" time at watermark bounded 1 min idle after 1 min;\nrule P intent 'x' from S select k, ts as at insert into TT;\n" +
			plain, want: "E-TIME-002"},
		{name: "second_rule_into_string_ts", rules: "stream B (k string, ts string);\n" +
			"rule A intent 'x' from S select k insert into B;\nrule C intent 'x' from R select k insert into B;\n" + plain,
			want: "E-TYPE-001 E-GRAPH-003"},
		{name: "http_sink_plain_rule", rules: "@sink(type = 'http', url = 'u') stream H (k string, ts timestamp);\n" +
			"rule P intent 'x' from S select k insert into H;\n" + plain},
		{name: "http_sink_batch_window", rules: "@sink(type = 'http', url = 'u') stream H (k string, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from S window batch() emit on batch end late events drop" +
			" select k insert into H;\n" + plain},
		{name: "http_sink_sort_window", rules: "@sink(type = 'http', url = 'u') stream H (k string, ts timestamp);\n" +
			"rule P intent 'x' partition by k of S ttl 1 hour from S window sort(3, n, desc) emit on change late events drop" +
			" select k insert into H;\n" + plain, want: "E-SINK-001"},
		// mayBeNull: the engine's flag; the rule's own partition key against another stream's key of the same name.
		// raw() reads the declared attribute, not the engine's flag: declared optional (itself E-TYPE-001), it can be null.
		{name: "raw_invisibles_declared_optional", rules: "stream Q (k string, text_invisibles bool optional, ts timestamp)" +
			clocked + "stream F (k string, f bool, ts timestamp);\n" +
			"rule P intent 'x' from Q select k, raw(text_invisibles) as f insert into F;\n" + plain,
			want: "E-TYPE-001 E-NULL-003", msg: "text_invisibles can be null and F.f is not optional"},
		{name: "invisibles_declared_optional", rules: "stream Q (k string, text_invisibles bool optional, ts timestamp)" +
			clocked + "stream F (k string, f bool, ts timestamp);\n" +
			"rule P intent 'x' from Q select k, text_invisibles as f insert into F;\n" + plain, want: "E-TYPE-001"},
		{name: "invisibles_select", rules: "stream F (k string, f bool, ts timestamp);\n" +
			"rule P intent 'x' from S select k, text_invisibles as f insert into F;\n" + plain},
		{name: "own_partition_key", rules: "stream A (u string optional, ts timestamp)" + clocked +
			"stream B (u string optional, v string, ts timestamp)" + clocked +
			"rule P intent 'x' partition by u of A, v of B ttl 1 hour from every a = A -> b = B" + patTail +
			"a.u as k insert into Out;\n"},
		{name: "other_streams_key_name", rules: "stream A (u string optional, ts timestamp)" + clocked +
			"stream B (u string optional, v string, ts timestamp)" + clocked +
			"rule P intent 'x' partition by u of A, v of B ttl 1 hour from every a = A -> b = B" + patTail +
			"b.u as k insert into Out;\n", want: "E-NULL-003"},
		// graph: G1-G7 over the time clause and the producers; a derived read.
		// A rule poisoned by its unknown sort attribute skips the derived-read pass, so its self-read is the cycle
		// finding; a timed stream with no producer (S, read) must not count as having explained it.
		{name: "cycle_through_a_poisoned_rule", rules: "stream X (k string, ts timestamp);\n" +
			"rule A intent 'x' partition by k of X ttl 1 hour from X window sort(3, nope, desc) emit on change late events drop" +
			" select k insert into X;\n" + plain, want: "E-NAME-001 E-GRAPH-001", msg: "rules A -> A form a cycle"},
		{name: "G1_untimed_unproduced", rules: "stream U (k string);\n" + plain, want: "E-TIME-001"},
		{name: "G2_untimed_one_producer", rules: plain},
		{name: "G3_untimed_two_producers", rules: "stream D (k string, ts timestamp);\n" +
			"rule A intent 'x' from S select k insert into D;\nrule B intent 'x' from R select k insert into D;\n" + plain,
			want: "E-GRAPH-003"},
		{name: "G4_timed_unproduced", rules: "stream Q (k string, ts timestamp)" + clocked + plain},
		{name: "G5_timed_produced_read", rules: "stream Q (k string, ts timestamp)" + clocked +
			"rule A intent 'x' from S select k insert into Q;\nrule B intent 'x' from Q select k insert into Out;\n",
			want: "E-GRAPH-002"},
		{name: "G6_timed_produced_unread_source", rules: "@source(type = 'stdin') stream Q (k string, ts timestamp)" +
			clocked + "rule A intent 'x' from S select k insert into Q;\n" + plain, want: "E-GRAPH-002"},
		{name: "G7_timed_produced_unread_sink", rules: "@sink(type = 'log') stream Q (k string, ts timestamp)" +
			clocked + "rule A intent 'x' from S select k insert into Q;\n" + plain, want: "E-TIME-002"},
		{name: "unproduced_untimed_read_is_not_derived", rules: "stream U (k string);\n" +
			"rule P intent 'x' from U select k insert into Out;\n", want: "E-TIME-001"},
		{name: "derived_read_twice", rules: "stream D (k string, ts timestamp);\nrule A intent 'x' from S select k insert into D;\n" +
			"rule B intent 'x' partition by k of D ttl 1 hour from every a = D -> b = D" + patTail + "a.k insert into Out;\n",
			want: "E-GRAPH-004"},
		// idle: no timer; a pattern without an absence; a written zero idle after; two timer rules on one stream.
		{name: "plain_over_clock_without_idle", rules: "stream Q (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
			"rule P intent 'x' from Q select k insert into Out;\n"},
		{name: "pattern_without_absence", rules: "stream Q (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
			"rule P intent 'x' partition by k of Q ttl 1 hour from every a = Q -> b = Q" + patTail + "a.k insert into Out;\n"},
		{name: "absence_over_one_stream", rules: "stream Q (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
			"rule P intent 'x' partition by k of Q ttl 1 hour from every a = Q and not Q[k == 'z'] for 1 min" + patTail +
			"a.k insert into Out;\n", want: "E-IDLE-001"},
		{name: "idle_after_zero", rules: "stream Q (k string, ts timestamp) time ts watermark bounded 1 min idle after 0 ms;\n" +
			"rule P intent 'x' partition by k of Q ttl 1 hour from Q window sort(3, k, desc) emit on change late events drop" +
			" select k insert into Out;\n", want: "E-DUR-003"},
		{name: "two_timer_rules_one_stream", rules: "stream Q (k string, ts timestamp) time ts watermark bounded 1 min;\n" +
			"stream O2 (k string, ts timestamp);\n" +
			"rule W intent 'x' partition by k of Q ttl 1 hour from Q window sort(3, k, desc) emit on change late events drop" +
			" select k insert into Out;\n" +
			"rule V intent 'x' partition by k of Q ttl 1 hour from Q window sort(3, k, desc) emit on change late events drop" +
			" select k insert into O2;\n", want: "E-IDLE-001"},
		// names: a sort window over an unknown stream is one finding, not a second for its sort attribute.
		{name: "sort_window_over_unknown_stream", rules: "rule P intent 'x' partition by k of S ttl 1 hour from Nope" +
			" window sort(3, k, desc) emit on change late events drop select k insert into Out;\n",
			want: "E-NAME-001", msg: "unknown stream Nope"},
		// resolveExpr: a literal that is no param; a text operand that is no param.
		{name: "literal_not_param", rules: "rule P intent 'x' from S[n > 1] select k insert into Out;\n"},
		{name: "text_operand_not_param", rules: "rule P intent 'x' from S[k has_ci 'x'] select k insert into Out;\n"},
		// resolveExpr: a keyword list with two empty items is one finding, at the first.
		{name: "two_empty_keyword_items", rules: "param kw string = 'a,,b,';\n" +
			"rule P intent 'x' from S[k has_any_ci(${kw})] select k insert into Out;\n", want: "E-PARAM-001",
			msg: "${kw} = \"a,,b,\": item 2 of the comma-separated list is empty", not: "item 4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fs := findings(t, "unit.cep", base+c.rules, c.opts)
			codes := make([]string, 0, len(fs))
			msgs := make([]string, 0, len(fs))
			for _, f := range fs {
				codes = append(codes, f.Code)
				msgs = append(msgs, f.Msg)
			}
			if strings.Join(codes, " ") != c.want {
				t.Fatalf("got %v, want [%s]", fs, c.want)
			}
			if c.msg != "" && !strings.Contains(strings.Join(msgs, "\n"), c.msg) {
				t.Fatalf("messages %q lack %q", msgs, c.msg)
			}
			if c.not != "" && strings.Contains(strings.Join(msgs, "\n"), c.not) {
				t.Fatalf("messages %q hold %q", msgs, c.not)
			}
		})
	}
	// idle: a model built in Go has no positions, so `idle after` is known by its value alone.
	// ttl: a model built in Go may carry a ttl with no partition key; with no key there is no ttl to check.
	t.Run("go_built_ttl_without_a_partition", func(t *testing.T) {
		t.Parallel()
		app := parse(t, base+"rule P intent 'x' from S select k output first every 5 min insert into Out;\n")
		app.Rules[0].Partition.TTL = model.DurationOrParam{Ms: 60_000}
		if _, diags := Check(app, Options{}); len(diags) != 0 {
			t.Fatalf("got %v, want none", diags)
		}
	})
	t.Run("go_built_idle_after_without_position", func(t *testing.T) {
		t.Parallel()
		app := parse(t, base+"rule P intent 'x' partition by k of S ttl 1 hour from S window sort(3, n, desc) emit on change"+
			" late events drop select k insert into Out;\n")
		clearPositions(reflect.ValueOf(app).Elem())
		if _, diags := Check(app, Options{}); len(diags) != 0 {
			t.Fatalf("got %v, want none", diags)
		}
	})
	// cycle: a rule reading one producer's output from two steps is one edge, so the cycle names each rule once.
	t.Run("cycle_two_steps_one_edge", func(t *testing.T) {
		t.Parallel()
		step := func(stream string) model.Step { return model.Step{Source: model.Source{Stream: stream}} }
		rules := []model.Rule{
			{Name: "A", Body: model.Body{Kind: model.BodyPlain, Source: model.Source{Stream: "S"}}, InsertInto: "X"},
			{Name: "B", Body: model.Body{Kind: model.BodyPattern, Pattern: model.Pattern{Elements: []model.PatternElement{
				{Steps: []model.Step{step("X"), step("X")}},
			}}}, InsertInto: "S"},
		}
		if got := cycle(rules); !reflect.DeepEqual(got, []int{0, 1}) {
			t.Fatalf("got %v, want [0 1]", got)
		}
	})
}
