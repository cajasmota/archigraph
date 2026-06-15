package php

// test_data.go — test target extraction and dependency graph for PHP test
// frameworks: Behat, Codeception, and Pest.
//
// PHPUnit (test.phpunit) is already covered by internal/engine/tests_edges.go
// and internal/engine/rules/php/test_patterns.yaml.
//
// Coverage cells driven to green by this file:
//   test.behat       : dependency_graph, target_extraction
//   test.codeception : dependency_graph, target_extraction
//   test.pest        : dependency_graph, target_extraction

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_php_behat_test", &behatTestExtractor{})
	extractor.Register("custom_php_codeception_test", &codeceptionTestExtractor{})
	extractor.Register("custom_php_pest_test", &pestTestExtractor{})
}

// ============================================================================
// Behat — .feature files + FeatureContext step definitions
// ============================================================================

type behatTestExtractor struct{}

func (e *behatTestExtractor) Language() string { return "custom_php_behat_test" }

var (
	// behatFeatureRe matches "Feature: <name>" lines in .feature files
	behatFeatureRe = regexp.MustCompile(`(?m)^Feature:\s*(.+)$`)

	// behatScenarioRe matches "Scenario: <name>" and "Scenario Outline: <name>"
	behatScenarioRe = regexp.MustCompile(`(?m)^(?:\s*)Scenario(?:\s+Outline)?:\s*(.+)$`)

	// behatStepRe matches Given/When/Then step annotations in PHP context classes.
	// Go regexp does not support backreferences; we match single/double quoted
	// patterns independently and pick up the pattern group.
	behatStepRe = regexp.MustCompile(
		`(?m)@(Given|When|Then|And|But)\s*\(\s*(?:'([^']+)'|"([^"]+)")`)

	// behatContextClassRe detects Behat context classes (implements Context)
	behatContextClassRe = regexp.MustCompile(
		`(?m)class\s+(\w+)\s+(?:extends\s+\w+\s+)?implements\s+(?:\w+\\)*Context\b`)

	// behatUsesRe detects RawMinkContext extension
	behatUsesRe = regexp.MustCompile(
		`(?m)class\s+(\w+)\s+extends\s+(?:\w+\\)*(?:RawMinkContext|MinkContext)\b`)
)

func (e *behatTestExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "php_behat_test.extract",
		trace.WithAttributes(
			attribute.String("file", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) { ormAdd(seen, &entities, ent) }

	isFeatureFile := len(file.Path) > 8 && file.Path[len(file.Path)-8:] == ".feature"
	isPHP := file.Language == "php"

	if !isFeatureFile && !isPHP {
		span.SetAttributes(attribute.Int("entity_count", 0))
		return nil, nil
	}

	if isFeatureFile {
		// 1. Feature declarations → SCOPE.Operation/test_suite
		for _, m := range behatFeatureRe.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity("feature:"+name, "SCOPE.Operation", "test_suite", file.Path, "gherkin", lineOf(src, m[0]))
			setProps(&ent, "framework", "behat", "provenance", "INFERRED_FROM_BEHAT_FEATURE",
				"feature_name", name)
			add(ent)
		}

		// 2. Scenario declarations → SCOPE.Operation/test_case
		for _, m := range behatScenarioRe.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity("scenario:"+name, "SCOPE.Operation", "test_case", file.Path, "gherkin", lineOf(src, m[0]))
			setProps(&ent, "framework", "behat", "provenance", "INFERRED_FROM_BEHAT_SCENARIO",
				"scenario_name", name)
			add(ent)
		}
	}

	if isPHP {
		// 3. Step definitions → SCOPE.Operation/test_step (target extraction)
		// behatStepRe groups: [0]=full [2:3]=keyword [4:5]=single-q [6:7]=double-q
		for _, m := range behatStepRe.FindAllStringSubmatchIndex(src, -1) {
			stepKw := src[m[2]:m[3]]
			// pick whichever quote group matched
			pattern := ""
			if m[4] >= 0 && m[5] >= 0 {
				pattern = src[m[4]:m[5]]
			} else if m[6] >= 0 && m[7] >= 0 {
				pattern = src[m[6]:m[7]]
			}
			if pattern == "" {
				continue
			}
			ent := makeEntity("step:"+stepKw+":"+pattern, "SCOPE.Operation", "test_step", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "behat", "provenance", "INFERRED_FROM_BEHAT_STEP",
				"step_keyword", stepKw, "step_pattern", pattern)
			add(ent)
		}

		// 4. Context classes → SCOPE.Component/test_context (dependency graph)
		for _, m := range behatContextClassRe.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity(name, "SCOPE.Component", "test_context", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "behat", "provenance", "INFERRED_FROM_BEHAT_CONTEXT")
			add(ent)
		}

		// 5. MinkContext subclasses → SCOPE.Component/test_context
		for _, m := range behatUsesRe.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity(name, "SCOPE.Component", "test_context", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "behat", "provenance", "INFERRED_FROM_BEHAT_MINK_CONTEXT")
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ============================================================================
// Codeception — Cest/Cept classes and module dependencies
// ============================================================================

type codeceptionTestExtractor struct{}

func (e *codeceptionTestExtractor) Language() string { return "custom_php_codeception_test" }

var (
	// codeceptionCestRe detects Cest class declarations
	codeceptionCestRe = regexp.MustCompile(
		`(?m)class\s+(\w+Cest)\b`)

	// codeceptionTestMethodRe detects public test methods in Cest classes.
	// Matches both untyped ($I) and typed (AcceptanceTester $I) actor params.
	codeceptionTestMethodRe = regexp.MustCompile(
		`(?m)public\s+function\s+(\w+)\s*\(\s*(?:\w+\s+)?\$I\b`)

	// codeceptionModuleRe detects module dependencies in suite configs or _support
	codeceptionModuleRe = regexp.MustCompile(
		`(?m)use\s+(Codeception\\Module\\[A-Za-z]+)\b`)

	// codeceptionActorRe detects the generated Actor class usage — both via
	// use statement and as a method parameter type hint.
	codeceptionActorRe = regexp.MustCompile(
		`(?m)(?:use\s+|\b)(AcceptanceTester|FunctionalTester|UnitTester)\s*(?:\b|;|\s+\$)`)

	// codeceptionExtendRe detects Codeception unit test classes
	codeceptionExtendRe = regexp.MustCompile(
		`(?m)class\s+(\w+)\s+extends\s+\\?Codeception\\Test\\Unit\b`)

	// codeceptionHaveRe detects $I->have... / $I->see... calls (target extraction)
	codeceptionHaveRe = regexp.MustCompile(
		`(?m)\$I->(have\w+|see\w+|am\w+|send\w+|grab\w+|expect\w+)\s*\(`)
)

func (e *codeceptionTestExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "php_codeception_test.extract",
		trace.WithAttributes(
			attribute.String("file", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "php" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) { ormAdd(seen, &entities, ent) }

	// Gate: must contain Codeception markers
	hasCept := codeceptionCestRe.FindStringIndex(src) != nil ||
		codeceptionExtendRe.FindStringIndex(src) != nil ||
		codeceptionActorRe.FindStringIndex(src) != nil
	if !hasCept {
		span.SetAttributes(attribute.Int("entity_count", 0))
		return nil, nil
	}

	// 1. Cest class → SCOPE.Component/test_suite (target extraction)
	for _, m := range codeceptionCestRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Component", "test_suite", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_CEST")
		add(ent)
	}

	// 2. Codeception unit test class → SCOPE.Component/test_suite
	for _, m := range codeceptionExtendRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Component", "test_suite", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_UNIT")
		add(ent)
	}

	// 3. Test methods in Cest class → SCOPE.Operation/test_case (target extraction)
	for _, m := range codeceptionTestMethodRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Operation", "test_case", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_TEST_METHOD")
		add(ent)
	}

	// 4. Module dependencies → SCOPE.Component/test_dependency (dependency graph)
	for _, m := range codeceptionModuleRe.FindAllStringSubmatchIndex(src, -1) {
		modName := src[m[2]:m[3]]
		ent := makeEntity(modName, "SCOPE.Component", "test_dependency", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_MODULE",
			"module_name", modName)
		add(ent)
	}

	// 5. Actor usage → SCOPE.Component/test_actor (dependency graph)
	for _, m := range codeceptionActorRe.FindAllStringSubmatchIndex(src, -1) {
		actorName := src[m[2]:m[3]]
		ent := makeEntity(actorName, "SCOPE.Component", "test_actor", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_ACTOR")
		add(ent)
	}

	// 6. $I->action calls → SCOPE.Operation/test_step (target extraction)
	for _, m := range codeceptionHaveRe.FindAllStringSubmatchIndex(src, -1) {
		action := src[m[2]:m[3]]
		ent := makeEntity("step:"+action, "SCOPE.Operation", "test_step", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "codeception", "provenance", "INFERRED_FROM_CODECEPTION_STEP",
			"action", action)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// ============================================================================
// Pest — functional test declarations (it/test/describe)
// ============================================================================

type pestTestExtractor struct{}

func (e *pestTestExtractor) Language() string { return "custom_php_pest_test" }

var (
	// pestTestRe matches it('...') / it("...") or test('...') / test("...") declarations.
	// Groups: [2:3]=single-q name, [4:5]=double-q name (backrefs unsupported in Go).
	pestTestRe = regexp.MustCompile(
		`(?m)^(?:it|test)\s*\(\s*(?:'([^']+)'|"([^"]+)")`)

	// pestDescribeRe matches describe('...') / describe("...") block declarations.
	// Groups: [2:3]=single-q, [4:5]=double-q.
	pestDescribeRe = regexp.MustCompile(
		`(?m)^describe\s*\(\s*(?:'([^']+)'|"([^"]+)")`)

	// pestUsesRe matches uses(ClassName::class) for test class dependencies
	pestUsesRe = regexp.MustCompile(
		`(?m)uses\s*\(\s*([A-Za-z_][A-Za-z0-9_\\]*::class)`)

	// pestDatasetRe matches dataset('...') / dataset("...") declarations.
	// Groups: [2:3]=single-q, [4:5]=double-q.
	pestDatasetRe = regexp.MustCompile(
		`(?m)dataset\s*\(\s*(?:'([^']+)'|"([^"]+)")`)

	// pestBeforeEachRe matches beforeEach/afterEach hooks
	pestBeforeEachRe = regexp.MustCompile(
		`(?m)^(beforeEach|afterEach|beforeAll|afterAll)\s*\(`)

	// pestExpectRe matches expect() chains (target extraction — what is being tested)
	pestExpectRe = regexp.MustCompile(
		`(?m)\bexpect\s*\(\s*(\$\w+|\w+\s*\(|new\s+\w+)`)

	// pestArchRe matches arch() tests (dependency graph — architectural constraints)
	pestArchRe = regexp.MustCompile(
		`(?m)\barch\s*\(\s*\)\s*->`)
)

func (e *pestTestExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "php_pest_test.extract",
		trace.WithAttributes(
			attribute.String("file", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "php" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) { ormAdd(seen, &entities, ent) }

	// Gate: must contain Pest function declarations
	hasPest := pestTestRe.FindStringIndex(src) != nil ||
		pestDescribeRe.FindStringIndex(src) != nil ||
		pestUsesRe.FindStringIndex(src) != nil
	if !hasPest {
		span.SetAttributes(attribute.Int("entity_count", 0))
		return nil, nil
	}

	// pickQuoted returns the first non-empty alternative from a regex match
	// where group [2:3] is the single-quote capture and [4:5] the double-quote.
	pickQuoted := func(src string, m []int) string {
		if m[2] >= 0 && m[3] >= 0 {
			return src[m[2]:m[3]]
		}
		if m[4] >= 0 && m[5] >= 0 {
			return src[m[4]:m[5]]
		}
		return ""
	}

	// 1. it('...') / test('...') → SCOPE.Operation/test_case (target extraction)
	for _, m := range pestTestRe.FindAllStringSubmatchIndex(src, -1) {
		name := pickQuoted(src, m)
		if name == "" {
			continue
		}
		ent := makeEntity(name, "SCOPE.Operation", "test_case", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_TEST")
		add(ent)
	}

	// 2. describe('...') → SCOPE.Component/test_suite (target extraction)
	for _, m := range pestDescribeRe.FindAllStringSubmatchIndex(src, -1) {
		name := pickQuoted(src, m)
		if name == "" {
			continue
		}
		ent := makeEntity("describe:"+name, "SCOPE.Component", "test_suite", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_DESCRIBE")
		add(ent)
	}

	// 3. uses(ClassName::class) → SCOPE.Component/test_dependency (dependency graph)
	for _, m := range pestUsesRe.FindAllStringSubmatchIndex(src, -1) {
		className := src[m[2]:m[3]]
		ent := makeEntity(className, "SCOPE.Component", "test_dependency", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_USES",
			"uses_class", className)
		add(ent)
	}

	// 4. dataset('...') → SCOPE.Schema/test_dataset (target extraction)
	for _, m := range pestDatasetRe.FindAllStringSubmatchIndex(src, -1) {
		name := pickQuoted(src, m)
		if name == "" {
			continue
		}
		ent := makeEntity("dataset:"+name, "SCOPE.Schema", "test_dataset", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_DATASET")
		add(ent)
	}

	// 5. beforeEach/afterEach hooks → SCOPE.Pattern/test_hook (dependency graph)
	for _, m := range pestBeforeEachRe.FindAllStringSubmatchIndex(src, -1) {
		hook := src[m[2]:m[3]]
		ent := makeEntity("hook:"+hook, "SCOPE.Pattern", "test_hook", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_HOOK",
			"hook_type", hook)
		add(ent)
	}

	// 6. arch() → SCOPE.Pattern/arch_test (dependency graph — architectural constraint)
	for _, m := range pestArchRe.FindAllStringIndex(src, -1) {
		ent := makeEntity("arch_test", "SCOPE.Pattern", "arch_test", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "pest", "provenance", "INFERRED_FROM_PEST_ARCH")
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
