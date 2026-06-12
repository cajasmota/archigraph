package tautology

import "testing"

func reasons(fs []Finding) map[Reason]int {
	m := map[Reason]int{}
	for _, f := range fs {
		m[f.Reason]++
	}
	return m
}

func TestJSTS_SelfCompare(t *testing.T) {
	r := Analyze(Input{
		Language:  "typescript",
		StartLine: 10,
		Source:    "expect(body.statusCounts).toEqual(body.statusCounts);",
	})
	if r.Verdict != VerdictIneffective {
		t.Fatalf("verdict = %s, want ineffective", r.Verdict)
	}
	if len(r.Findings) != 1 || r.Findings[0].Reason != ReasonSelfCompare {
		t.Fatalf("findings = %+v, want one self_compare", r.Findings)
	}
	if r.Findings[0].Line != 10 {
		t.Fatalf("line = %d, want 10", r.Findings[0].Line)
	}
}

func TestJSTS_ConstantTrue(t *testing.T) {
	r := Analyze(Input{Language: "javascript", Source: "  expect(true).toBe(true);"})
	if r.Verdict != VerdictIneffective || reasons(r.Findings)[ReasonConstantTrue] != 1 {
		t.Fatalf("want constant_true ineffective, got %s %+v", r.Verdict, r.Findings)
	}
}

func TestJSTS_SameLiteral(t *testing.T) {
	r := Analyze(Input{Language: "ts", Source: `expect("ok").toBe("ok");`})
	if reasons(r.Findings)[ReasonSameLiteral] != 1 {
		t.Fatalf("want same_literal, got %+v", r.Findings)
	}
}

func TestJSTS_AssertBodyContract_SelfCompare(t *testing.T) {
	r := Analyze(Input{Language: "typescript", Source: "assertBodyContract(res.body, res.body);"})
	if r.Verdict != VerdictIneffective || reasons(r.Findings)[ReasonSelfCompare] != 1 {
		t.Fatalf("want self_compare, got %s %+v", r.Verdict, r.Findings)
	}
}

func TestJSTS_RealAssertion_NotFlagged(t *testing.T) {
	src := `
expect(res.status).toBe(200);
expect(body.statusCounts).toEqual(expected.statusCounts);
expect(user.isActive).toBe(true);
`
	r := Analyze(Input{Language: "typescript", Source: src})
	if r.Verdict != VerdictEffective {
		t.Fatalf("real assertions flagged: %s %+v", r.Verdict, r.Findings)
	}
}

func TestJSTS_CommentedOutAssertionIgnored(t *testing.T) {
	r := Analyze(Input{Language: "typescript", Source: "// expect(true).toBe(true);"})
	if r.Verdict != VerdictEffective {
		t.Fatalf("commented assertion flagged: %+v", r.Findings)
	}
}

func TestPython_AssertTrue(t *testing.T) {
	for _, src := range []string{"    assert True", "    self.assertTrue(True)"} {
		r := Analyze(Input{Language: "python", Source: src})
		if r.Verdict != VerdictIneffective {
			t.Fatalf("python %q not flagged: %s", src, r.Verdict)
		}
	}
}

func TestPython_SelfCompareAndReal(t *testing.T) {
	r := Analyze(Input{Language: "python", Source: "    assert resp.json() == resp.json()"})
	if reasons(r.Findings)[ReasonSelfCompare] != 1 {
		t.Fatalf("want self_compare, got %+v", r.Findings)
	}
	r2 := Analyze(Input{Language: "python", Source: "    assert resp.status_code == 200"})
	if r2.Verdict != VerdictEffective {
		t.Fatalf("real python assert flagged: %+v", r2.Findings)
	}
}

func TestGo_AssertEqualSelfCompare(t *testing.T) {
	r := Analyze(Input{Language: "go", Source: "\tassert.Equal(t, got.Body, got.Body)"})
	if reasons(r.Findings)[ReasonSelfCompare] != 1 {
		t.Fatalf("want self_compare, got %+v", r.Findings)
	}
}

func TestJava_AssertEqualsSameLiteral(t *testing.T) {
	r := Analyze(Input{Language: "java", Source: "        assertEquals(200, 200);"})
	if reasons(r.Findings)[ReasonSameLiteral] != 1 {
		t.Fatalf("want same_literal, got %+v", r.Findings)
	}
}

func TestRuby_SelfCompare(t *testing.T) {
	r := Analyze(Input{Language: "ruby", Source: "    expect(subject.body).to eq(subject.body)"})
	if reasons(r.Findings)[ReasonSelfCompare] != 1 {
		t.Fatalf("want self_compare, got %+v", r.Findings)
	}
}

func TestUnsupportedLanguage(t *testing.T) {
	r := Analyze(Input{Language: "cobol", Source: "anything"})
	if r.Verdict != VerdictUnknown || r.Supported {
		t.Fatalf("unsupported lang: %s supported=%v", r.Verdict, r.Supported)
	}
}

func TestNoGoldenLinkage(t *testing.T) {
	// Source references nothing from the linkage terms.
	r := Analyze(Input{
		Language:     "typescript",
		Source:       "expect(res.status).toBe(200);",
		LinkageTerms: []string{"createOrder", "/api/orders"},
	})
	if !r.NoGoldenLinkage {
		t.Fatalf("expected no_golden_linkage advisory")
	}
	if r.Verdict != VerdictEffective {
		t.Fatalf("no_golden_linkage must not make verdict ineffective: %s", r.Verdict)
	}
	// Now referencing one term clears the advisory.
	r2 := Analyze(Input{
		Language:     "typescript",
		Source:       "createOrder(); expect(res.status).toBe(200);",
		LinkageTerms: []string{"createOrder", "/api/orders"},
	})
	if r2.NoGoldenLinkage {
		t.Fatalf("linkage term present, advisory should be off")
	}
}

func TestMultipleFindingsSorted(t *testing.T) {
	src := "expect(true).toBe(true);\nassertBodyContract(b, b);"
	r := Analyze(Input{Language: "typescript", StartLine: 5, Source: src})
	SortFindings(r.Findings)
	if len(r.Findings) != 2 {
		t.Fatalf("want 2 findings, got %+v", r.Findings)
	}
	if r.Findings[0].Line != 5 || r.Findings[1].Line != 6 {
		t.Fatalf("lines not absolute/sorted: %+v", r.Findings)
	}
}
