package substrate

import "testing"

// TestAnalyzeBranchesJSTS_Outcomes exercises the JS/TS classifier on a service
// method with an env-gate, an early-return guard, a status-returning guard, and
// a try/catch that re-throws.
func TestAnalyzeBranchesJSTS_Outcomes(t *testing.T) {
	src := `async function createUser(req, res) {
    if (!process.env.SIGNUP_ENABLED) {
        return res.status(503).json({ error: "disabled" });
    }
    if (req.body.email == null) {
        return res.status(400).json({ error: "email required" });
    }
    try {
        const u = await db.users.create(req.body);
        return res.status(201).json(u);
    } catch (e) {
        logger.error(e);
        throw new HttpException("create failed", 500);
    }
}`
	br := analyzeBranchesJSTS(src, 1)
	if len(br) != 3 {
		t.Fatalf("expected 3 branches, got %d: %+v", len(br), br)
	}

	// env-gate
	if br[0].Kind != BranchEnvGate || br[0].EnvVar != "SIGNUP_ENABLED" {
		t.Errorf("branch0 = %+v; want env_gate SIGNUP_ENABLED", br[0])
	}
	if br[0].Outcome != OutcomeReturnValue {
		t.Errorf("branch0 outcome = %v; want return_value", br[0].Outcome)
	}
	if br[0].Returns == nil || br[0].Returns.Status != "503" {
		t.Errorf("branch0 returns = %+v; want 503", br[0].Returns)
	}

	// guard (the env-gate above consumed the leading-guard slot, mirroring the
	// Python analyzer — later guards are mid-body `guard`).
	if br[1].Kind != BranchGuard || br[1].Outcome != OutcomeReturnValue {
		t.Errorf("branch1 = %+v; want guard/return_value", br[1])
	}
	if br[1].Returns == nil || br[1].Returns.Status != "400" {
		t.Errorf("branch1 returns = %+v; want 400", br[1].Returns)
	}

	// catch that re-throws → raise, status 500 from HttpException
	if br[2].Kind != BranchExcept || br[2].Outcome != OutcomeRaise {
		t.Errorf("branch2 = %+v; want except/raise", br[2])
	}
	if br[2].Returns == nil || br[2].Returns.Status != "500" {
		t.Errorf("branch2 returns = %+v; want 500", br[2].Returns)
	}
}

// TestAnalyzeBranchesJSTS_Swallow confirms a log-only catch is swallow.
func TestAnalyzeBranchesJSTS_Swallow(t *testing.T) {
	src := `function f() {
    try {
        doThing();
    } catch (err) {
        console.warn("ignored", err);
    }
}`
	br := analyzeBranchesJSTS(src, 1)
	if len(br) != 1 || br[0].Kind != BranchExcept || br[0].Outcome != OutcomeSwallow {
		t.Fatalf("expected one swallow except, got %+v", br)
	}
}

// TestAnalyzeBranchesJava_Outcomes exercises the Java classifier on a Spring
// controller method with an env-gate (@Value is not in-body so we test
// System.getenv), an early-return guard returning ResponseEntity.status, and a
// try/catch that returns a 500.
func TestAnalyzeBranchesJava_Outcomes(t *testing.T) {
	src := `public ResponseEntity<?> create(@RequestBody Dto dto) {
    if (System.getenv("SIGNUP_ENABLED") == null) {
        return ResponseEntity.status(503).build();
    }
    if (dto.getEmail() == null) {
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).build();
    }
    try {
        repo.save(dto);
        return ResponseEntity.status(HttpStatus.CREATED).build();
    } catch (Exception e) {
        log.error("failed", e);
        return ResponseEntity.status(500).build();
    }
}`
	br := analyzeBranchesJava(src, 1)
	if len(br) != 3 {
		t.Fatalf("expected 3 branches, got %d: %+v", len(br), br)
	}

	if br[0].Kind != BranchEnvGate || br[0].EnvVar != "SIGNUP_ENABLED" {
		t.Errorf("branch0 = %+v; want env_gate SIGNUP_ENABLED", br[0])
	}
	if br[0].Returns == nil || br[0].Returns.Status != "503" {
		t.Errorf("branch0 returns = %+v; want 503", br[0].Returns)
	}

	if br[1].Kind != BranchGuard || br[1].Outcome != OutcomeReturnValue {
		t.Errorf("branch1 = %+v; want guard/return_value", br[1])
	}
	if br[1].Returns == nil || br[1].Returns.Status != "400" {
		t.Errorf("branch1 returns = %+v; want 400 (HttpStatus.BAD_REQUEST)", br[1].Returns)
	}

	if br[2].Kind != BranchExcept || br[2].Outcome != OutcomeReturnValue {
		t.Errorf("branch2 = %+v; want except/return_value", br[2])
	}
	if br[2].Returns == nil || br[2].Returns.Status != "500" {
		t.Errorf("branch2 returns = %+v; want 500", br[2].Returns)
	}
}

// TestAnalyzeBranchesJava_Throw confirms a catch that re-throws is raise.
func TestAnalyzeBranchesJava_Throw(t *testing.T) {
	src := `public void f() {
    try {
        risky();
    } catch (IOException e) {
        throw new RuntimeException(e);
    }
}`
	br := analyzeBranchesJava(src, 1)
	if len(br) != 1 || br[0].Kind != BranchExcept || br[0].Outcome != OutcomeRaise {
		t.Fatalf("expected one raise except, got %+v", br)
	}
}

// TestAnalyzeBranchesGo_Outcomes exercises the Go classifier on a handler with
// an env-gate, the dominant `if err != nil { return ... }` guard, an
// http.Error status branch, and a panic.
func TestAnalyzeBranchesGo_Outcomes(t *testing.T) {
	src := `func handler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("FEATURE_X") == "" {
		http.Error(w, "disabled", 503)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad", http.StatusBadRequest)
		return
	}
	if body == nil {
		panic("nil body")
	}
}`
	br := analyzeBranchesGo(src, 1)
	if len(br) != 3 {
		t.Fatalf("expected 3 branches, got %d: %+v", len(br), br)
	}

	// env-gate
	if br[0].Kind != BranchEnvGate || br[0].EnvVar != "FEATURE_X" {
		t.Errorf("branch0 = %+v; want env_gate FEATURE_X", br[0])
	}
	if br[0].Outcome != OutcomeReturnValue {
		t.Errorf("branch0 outcome = %v; want return_value", br[0].Outcome)
	}
	if br[0].Returns == nil || br[0].Returns.Status != "503" {
		t.Errorf("branch0 returns = %+v; want 503", br[0].Returns)
	}

	// err != nil guard (the env-gate consumed the leading slot → guard)
	if br[1].Kind != BranchGuard || br[1].Outcome != OutcomeReturnValue {
		t.Errorf("branch1 = %+v; want guard/return_value", br[1])
	}
	if br[1].Condition != "if err != nil" {
		t.Errorf("branch1 condition = %q; want `if err != nil`", br[1].Condition)
	}
	if br[1].Returns == nil || br[1].Returns.Status != "400" {
		t.Errorf("branch1 returns = %+v; want 400 (http.StatusBadRequest)", br[1].Returns)
	}

	// panic → raise
	if br[2].Outcome != OutcomeRaise {
		t.Errorf("branch2 = %+v; want raise (panic)", br[2])
	}
}

// TestAnalyzeBranchesGo_ErrInit confirms an `if err := f(); err != nil {`
// init-statement guard surfaces the boolean condition only.
func TestAnalyzeBranchesGo_ErrInit(t *testing.T) {
	src := `func f() error {
	if err := do(); err != nil {
		return fmt.Errorf("do: %w", err)
	}
	return nil
}`
	br := analyzeBranchesGo(src, 1)
	if len(br) != 1 {
		t.Fatalf("expected 1 branch, got %d: %+v", len(br), br)
	}
	if br[0].Condition != "if err != nil" {
		t.Errorf("condition = %q; want `if err != nil`", br[0].Condition)
	}
	if br[0].Outcome != OutcomeReturnValue {
		t.Errorf("outcome = %v; want return_value", br[0].Outcome)
	}
}

// TestBranchAnalyzerRegistry_BraceLangs confirms jsts/java/go are registered.
func TestBranchAnalyzerRegistry_BraceLangs(t *testing.T) {
	for _, lang := range []string{"jsts", "java", "go"} {
		if BranchAnalyzerFor(lang) == nil {
			t.Errorf("%s branch analyzer not registered", lang)
		}
	}
}

// TestAnalyzeBranchesJSTS_NestExceptionStatus verifies NestJS built-in HTTP
// exception classes and HttpStatus enum references decode to their fixed status
// in the response-branch facet (#4601 — so a 409/404-branching Nest handler
// surfaces those statuses without a numeric literal).
func TestAnalyzeBranchesJSTS_NestExceptionStatus(t *testing.T) {
	src := `async create(dto) {
    if (existing) {
        throw new ConflictException('taken');
    }
    if (!dto.name) {
        throw new NotFoundException('missing');
    }
    if (bad) {
        throw new HttpException('x', HttpStatus.UNPROCESSABLE_ENTITY);
    }
    return this.repo.create(dto);
}`
	br := analyzeBranchesJSTS(src, 1)
	want := map[string]bool{"409": false, "404": false, "422": false}
	for _, b := range br {
		if b.Returns != nil {
			if _, ok := want[b.Returns.Status]; ok {
				want[b.Returns.Status] = true
			}
		}
	}
	for status, seen := range want {
		if !seen {
			t.Errorf("expected a branch with status %s; got %+v", status, br)
		}
	}
}

// TestNestExceptionStatus_Mapping spot-checks the NestJS exception → status map.
func TestNestExceptionStatus_Mapping(t *testing.T) {
	cases := map[string]string{
		"BadRequestException":          "400",
		"UnauthorizedException":        "401",
		"ForbiddenException":           "403",
		"NotFoundException":            "404",
		"ConflictException":            "409",
		"UnprocessableEntityException": "422",
		"InternalServerErrorException": "500",
		"SomeUserDefinedException":     "",
	}
	for class, want := range cases {
		if got := nestExceptionStatus(class); got != want {
			t.Errorf("nestExceptionStatus(%q) = %q, want %q", class, got, want)
		}
	}
}
