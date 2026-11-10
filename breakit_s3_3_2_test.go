package cep_test

// QA break-it of S3.3-2 through the facade (design s3-3-2 §3.1, E-EXP-002). Prefix: bk.

import "testing"

// TestBreakitRequiredObjectAttributeAllowsABlock: a source stream may declare a required `object` attribute (it
// deploys without a block), but no literal is an object (X3), null is refused for a required attribute (X4, written)
// and so is leaving it out (X4, omitted): no expect block of any rule reading the stream can ever pass E-EXP-002,
// though SPEC §6.8 says rules shipped in packs must carry one. Which remedy (an omitted object given as an empty
// object, an object literal in blocks, or a refusal at the declaration) is the owner's; this test asks only that a
// block giving every scalar attribute deploys.
func TestBreakitRequiredObjectAttributeAllowsABlock(t *testing.T) {
	t.Parallel()
	const src = "language cep/0.1;\n\n" +
		"stream S (k string, doc object, ts timestamp)\n  time ts watermark bounded 1 min;\n\n" +
		"stream O (k string, ts timestamp);\n\n" +
		"rule R\n  intent 'Pass every key.'\n  from S\n" +
		"  expect given S [ {k: 'a', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
		"         then O [ {k: 'a'} ]\n" +
		"  select k\n  insert into O;\n"
	if _, err := exDeploy(t, src); err != nil {
		t.Errorf("Deploy: %v; want it deployed (a rule reading a stream with a required object attribute can carry a block)", err)
	}
}

// TestBreakitNonUTF8StringInABlockIsRefused: SPEC §2.1 says source is UTF-8, yet a string literal holding a raw 0xFF
// byte lexes, and a block giving it passes: the filter and the then-record compare the same malformed bytes. No live
// event can carry them (a payload's malformed UTF-8 becomes U+FFFD; a Go entry point refuses it: decisions.md,
// 2026-11-06, S3.2b-2's design, 1), so the block proves a path the deployed rule can never take. Deploy must refuse
// the source.
func TestBreakitNonUTF8StringInABlockIsRefused(t *testing.T) {
	t.Parallel()
	const bad = "\xff"
	src := "language cep/0.1;\n\n" +
		"stream S (k string, ts timestamp)\n  time ts watermark bounded 1 min;\n\n" +
		"stream O (k string, ts timestamp);\n\n" +
		"rule R\n  intent 'Pass the one key.'\n  from S[k == '" + bad + "']\n" +
		"  expect given S [ {k: '" + bad + "', ts: timestamp '2026-01-01T10:00:00Z'} ]\n" +
		"         then O [ {k: '" + bad + "'} ]\n" +
		"  select k\n  insert into O;\n"
	if _, err := exDeploy(t, src); err == nil {
		t.Errorf("Deploy of a source holding a 0xFF byte in its string literals: deployed, its block passing; want it refused (SPEC 2.1: source is UTF-8)")
	}
}
