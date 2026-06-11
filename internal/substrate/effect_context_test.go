package substrate

import "testing"

// TestEffectContextsPython proves conditional vs unconditional + loop
// attribution and the cyclomatic-complexity count on a small Django-shaped
// sample. The function makes a top-level db_read, a db_write guarded by an
// `if`, and an http_out inside a `for` loop over a collection.
func TestEffectContextsPython(t *testing.T) {
	src := `def sync(self, request):
    rows = Contact.objects.filter(active=True)
    if request.data.get("force"):
        Contact.objects.create(name="x")
    for row in rows:
        requests.post("https://api.example.com/notify", json={"id": row.id})
    return Response({"ok": True})
`
	ctxs, cx := EffectContextsFor("python", src, 100)

	if cx.Cyclomatic < 3 { // if + for + (1) at minimum
		t.Errorf("cyclomatic = %d; want >= 3", cx.Cyclomatic)
	}
	if cx.BranchCount != cx.Cyclomatic-1 {
		t.Errorf("branch_count %d != cyclomatic-1 %d", cx.BranchCount, cx.Cyclomatic-1)
	}

	byEffect := map[string]EffectContext{}
	for _, c := range ctxs {
		// keep the first occurrence per effect for the assertions below
		if _, ok := byEffect[c.Effect]; !ok {
			byEffect[c.Effect] = c
		}
	}

	read, ok := byEffect["db_read"]
	if !ok {
		t.Fatalf("no db_read effect detected; got %+v", ctxs)
	}
	if read.Conditional {
		t.Errorf("db_read should be unconditional (top-level), got %+v", read)
	}

	write, ok := byEffect["db_write"]
	if !ok {
		t.Fatalf("no db_write effect detected; got %+v", ctxs)
	}
	if !write.Conditional {
		t.Errorf("db_write should be conditional, got %+v", write)
	}
	if write.Condition == "" {
		t.Errorf("db_write should carry its guarding condition, got %+v", write)
	}
	if write.InLoop {
		t.Errorf("db_write is not in a loop, got %+v", write)
	}

	http, ok := byEffect["http_out"]
	if !ok {
		t.Fatalf("no http_out effect detected; got %+v", ctxs)
	}
	if !http.InLoop {
		t.Errorf("http_out should be in_loop (fan-out), got %+v", http)
	}
	if !http.Conditional {
		t.Errorf("http_out inside for-loop should be conditional, got %+v", http)
	}
}

// TestEffectContextsJSTS proves the same on a small NestJS-shaped TS sample.
func TestEffectContextsJSTS(t *testing.T) {
	src := `async function sync(req: Request) {
  const rows = await this.repo.find();
  if (req.body.force) {
    await this.repo.save({ name: 'x' });
  }
  for (const row of rows) {
    await fetch('https://api.example.com/notify', { method: 'POST' });
  }
  return { ok: true };
}
`
	ctxs, cx := EffectContextsFor("jsts", src, 50)

	if cx.Cyclomatic < 3 {
		t.Errorf("cyclomatic = %d; want >= 3", cx.Cyclomatic)
	}

	var sawCondWrite, sawLoopHTTP bool
	for _, c := range ctxs {
		switch c.Effect {
		case "db_write":
			if c.Conditional && c.Condition != "" {
				sawCondWrite = true
			}
		case "http_out":
			if c.InLoop {
				sawLoopHTTP = true
			}
		}
	}
	if !sawCondWrite {
		t.Errorf("expected a conditional db_write with a condition; got %+v", ctxs)
	}
	if !sawLoopHTTP {
		t.Errorf("expected an http_out inside a loop; got %+v", ctxs)
	}
}

// TestComputeFunctionComplexity covers the bare counter: a branchless function
// is complexity 1; decision points each add one.
func TestComputeFunctionComplexity(t *testing.T) {
	if got := ComputeFunctionComplexity("return 1").Cyclomatic; got != 1 {
		t.Errorf("branchless complexity = %d; want 1", got)
	}
	src := "if a {} for x {} while y {} a ? b : c"
	cx := ComputeFunctionComplexity(src)
	if cx.Cyclomatic != cx.BranchCount+1 {
		t.Errorf("invariant broken: %+v", cx)
	}
	if cx.BranchCount < 4 { // if, for, while, ternary
		t.Errorf("branch_count = %d; want >= 4", cx.BranchCount)
	}
}
