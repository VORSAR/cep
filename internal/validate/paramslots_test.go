package validate

import (
	"slices"
	"testing"

	"github.com/vorsar/cep/internal/model"
)

// TestCheckResolvesEveryParamSlot: the App Check returns holds every ${param} slot's value (design s1-1c-6 §5 §0 (O)):
// a filter literal, a single text operand, a keyword set split at commas, a duration slot and an annotation value.
func TestCheckResolvesEveryParamSlot(t *testing.T) {
	t.Parallel()
	src := base + "param word string = 'w, x';\nparam w duration = 5 min;\nparam hook string = 'http://x';\n" +
		"@sink(type = 'http', url = ${hook}) stream H (k string, ts timestamp);\n" +
		"stream O2 (k string, ts timestamp);\nstream O3 (k string, ts timestamp);\n" +
		"rule A intent 'x' from S[n > ${lim}] select k insert into Out;\n" +
		"rule B intent 'x' from S[k has_ci ${word}] select k insert into H;\n" +
		"rule C intent 'x' from S[k has_any_ci(${terms})] select k insert into O2;\n" +
		"rule D intent 'x' partition by k of S ttl 1 hour from every a = S within ${w}" +
		" after match skip past last late events drop select a.k insert into O3;\n"
	out, diags := Check(parse(t, src), Options{})
	if len(diags) != 0 || out == nil {
		t.Fatalf("got %v, app %v", diags, out != nil)
	}
	if lit := out.Rules[0].Body.Source.Filter.Args[1].Lit; lit != (model.Literal{Kind: model.LitInt, Int: 5}) {
		t.Errorf("the filter literal ${lim}: %+v, want int 5", lit)
	}
	if op := out.Rules[1].Body.Source.Filter.Operand; !slices.Equal(op.Strings, []string{"w, x"}) || op.Param != "" {
		t.Errorf("the text operand ${word}: %+v, want [w, x] (one item: a single operand is never split) and no param", op)
	}
	if op := out.Rules[2].Body.Source.Filter.Operand; !slices.Equal(op.Strings, []string{"a", "b"}) || op.Param != "" {
		t.Errorf("the keyword set ${terms}: %+v, want [a b] and no param", op)
	}
	if d := out.Rules[3].Body.Pattern.Within; d.Ms != 300_000 || d.Param != "" {
		t.Errorf("the duration slot ${w}: %+v, want 300000 ms and no param", d)
	}
	var url model.Literal
	for _, s := range out.Streams {
		if s.Name == "H" {
			url = s.Annotations[0].Items[1].Value
		}
	}
	if url != (model.Literal{Kind: model.LitString, Str: "http://x"}) {
		t.Errorf("the annotation value ${hook}: %+v, want string http://x", url)
	}
}
