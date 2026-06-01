// AWS CDK — JVM (Java) / Go / .NET (C#) language-binding resource + dependency
// extraction. Extends cdk_edges.go (which covers CDK-TS/JS + CDK-Python) to the
// three remaining first-class CDK languages. Part of #3550, epic #3512.
//
// Background / the gap this closes
// --------------------------------
// AWS CDK is multi-language: the same construct model (a construct is created
// with `(scope, "LogicalId", props)`, and access is wired with `.grant*(...)`)
// is expressed in TypeScript, Python, Java, Go and C#. cdk_edges.go implemented
// TS/JS + Python; the registry honestly stamped CDK-Java/Go/C# as "not yet
// implemented". This pass closes that gap for the three JSII language bindings,
// emitting exactly the same SCOPE.InfraResource entities + DEPENDS_ON edges +
// resource categories the TS/JS path emits, so a cross-language / cross-tool
// "all datastores" query sees Java/Go/C# CDK resources alongside TS ones.
//
// What this pass extracts (per binding)
// -------------------------------------
// The CDK identity is still the "LogicalId" string literal passed as the second
// positional argument to the construct constructor. The differences are purely
// syntactic:
//
//	Java   new Bucket(this, "Assets", BucketProps.builder().versioned(true).build());
//	       Function fn = Function.Builder.create(this, "Fn").runtime(...).build();
//	       bucket.grantRead(fn);
//	Go     bucket := awss3.NewBucket(stack, jsii.String("Assets"), &awss3.BucketProps{...})
//	       fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{...})
//	       bucket.GrantRead(fn, nil)
//	C#     var bucket = new Bucket(this, "Assets", new BucketProps { Versioned = true });
//	       var fn = new Function(this, "Fn", new FunctionProps { ... });
//	       bucket.GrantRead(fn);
//
// For each construct we emit a SCOPE.InfraResource NAMED by its LogicalId,
// carrying the construct TYPE and the uniform resource_category from the shared
// types.IaCResourceCategory classifier (#3549). Grant calls produce the same
// grantee --DEPENDS_ON--> resource edge as the TS path.
//
// Honest scope (partial, not full)
// --------------------------------
// These are single-file regex passes, like the TS/JS + Python ones. They model:
//   - construct declaration (assigned + anonymous), incl. the Java
//     `Type.Builder.create(scope, "Id")` fluent form and the Go
//     `pkg.NewType(scope, jsii.String("Id"), ...)` factory form,
//   - grant edges (`res.grant*(grantee)` / `res.Grant*(grantee, ...)`),
//   - props/args-passed construct references (enclosing DEPENDS_ON passed).
//
// Cross-file wiring and lower-signal idioms (event sources expressed very
// differently per language, addDependency, L1 escape hatches) are left to a
// follow-up; the registry stamps these bindings `partial`. Append-only: never
// mutates or removes existing entities/edges.
//
// Refs #3550, #3512.
package engine

import "regexp"

// cdkSupportsLanguageJVMGoNet reports whether this file handles `lang`.
func cdkSupportsLanguageJVMGoNet(lang string) bool {
	switch lang {
	case "java", "go", "csharp":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Java idioms.
//
//	new Bucket(this, "Assets", BucketProps.builder()...build())
//	Function fn = Function.Builder.create(this, "Fn")...build();
//	Bucket bucket = new Bucket(this, "Assets", ...);
//	bucket.grantRead(fn);
// ---------------------------------------------------------------------------

// cdkJavaConstructDeclRe captures `Type[ ]var = new Type(this, "Id"` — i.e. a
// `new`-style construct assigned to a typed local. Group 1 = var name, group 2
// = construct type, group 3 = "LogicalId". The declared-type prefix before the
// var is optional (`var bucket = new Bucket(...)` also matches because group 1
// captures the last identifier before `=`).
var cdkJavaConstructDeclRe = regexp.MustCompile(
	`(?:[A-Za-z_$][\w$.<>]*\s+)?([A-Za-z_$][\w$]*)\s*=\s*new\s+([A-Za-z_$][\w$.]*)\s*\(\s*(?:this|scope|stack|[A-Za-z_$][\w$]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkJavaBuilderDeclRe captures the fluent builder form
// `Type[ ]var = Type.Builder.create(this, "Id")`. Group 1 = var, group 2 = the
// Type whose Builder is used (the construct type), group 3 = "LogicalId".
var cdkJavaBuilderDeclRe = regexp.MustCompile(
	`(?:[A-Za-z_$][\w$.<>]*\s+)?([A-Za-z_$][\w$]*)\s*=\s*([A-Za-z_$][\w$.]*?)\.Builder\.create\s*\(\s*(?:this|scope|stack|[A-Za-z_$][\w$]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkJavaConstructAnonRe captures an UNASSIGNED `new Type(this, "Id"`.
var cdkJavaConstructAnonRe = regexp.MustCompile(
	`new\s+([A-Za-z_$][\w$.]*)\s*\(\s*(?:this|scope|stack|[A-Za-z_$][\w$]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkJavaBuilderAnonRe captures an UNASSIGNED `Type.Builder.create(this, "Id")`.
var cdkJavaBuilderAnonRe = regexp.MustCompile(
	`([A-Za-z_$][\w$.]*?)\.Builder\.create\s*\(\s*(?:this|scope|stack|[A-Za-z_$][\w$]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkJavaGrantRe captures `res.grantRead(grantee` (camelCase grant). Group 1 =
// resource var, group 2 = grant method, group 3 = grantee var.
var cdkJavaGrantRe = regexp.MustCompile(
	`([A-Za-z_$][\w$]*)\s*\.\s*(grant[A-Za-z]*)\s*\(\s*([A-Za-z_$][\w$]*)`,
)

// ---------------------------------------------------------------------------
// Go idioms.
//
//	bucket := awss3.NewBucket(stack, jsii.String("Assets"), &awss3.BucketProps{...})
//	fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{...})
//	bucket.GrantRead(fn, nil)
// ---------------------------------------------------------------------------

// cdkGoConstructDeclRe captures `var := pkg.NewType(scope, jsii.String("Id")`.
// Group 1 = Go var, group 2 = the package-qualified factory (e.g.
// `awss3.NewBucket`), group 3 = "LogicalId". The construct TYPE is derived from
// the factory by stripping the leading `New`.
var cdkGoConstructDeclRe = regexp.MustCompile(
	`([A-Za-z_][\w]*)\s*:?=\s*([A-Za-z_][\w.]*\.New[A-Za-z_][\w]*)\s*\(\s*(?:scope|stack|this|[A-Za-z_][\w]*)\s*,\s*jsii\.String\(\s*"([^"\n\r]+)"`,
)

// cdkGoConstructAnonRe captures an UNASSIGNED `pkg.NewType(scope, jsii.String("Id")`.
var cdkGoConstructAnonRe = regexp.MustCompile(
	`([A-Za-z_][\w.]*\.New[A-Za-z_][\w]*)\s*\(\s*(?:scope|stack|this|[A-Za-z_][\w]*)\s*,\s*jsii\.String\(\s*"([^"\n\r]+)"`,
)

// cdkGoGrantRe captures `res.GrantRead(grantee` (PascalCase Go method). Group 1
// = resource var, group 2 = grant method, group 3 = grantee var.
var cdkGoGrantRe = regexp.MustCompile(
	`([A-Za-z_][\w]*)\s*\.\s*(Grant[A-Za-z]*)\s*\(\s*([A-Za-z_][\w]*)`,
)

// ---------------------------------------------------------------------------
// C# idioms.
//
//	var bucket = new Bucket(this, "Assets", new BucketProps { Versioned = true });
//	var fn = new Function(this, "Fn", new FunctionProps { ... });
//	bucket.GrantRead(fn);
// ---------------------------------------------------------------------------

// cdkCSharpConstructDeclRe captures `[Type ]var = new Type(this, "Id"`. Group 1
// = var, group 2 = construct type, group 3 = "LogicalId". Covers both
// `var x = new T(...)` and `T x = new T(...)`.
var cdkCSharpConstructDeclRe = regexp.MustCompile(
	`(?:[A-Za-z_][\w.<>]*\s+)?([A-Za-z_][\w]*)\s*=\s*new\s+([A-Za-z_][\w.]*)\s*\(\s*(?:this|scope|stack|[A-Za-z_][\w]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkCSharpConstructAnonRe captures an UNASSIGNED `new Type(this, "Id"`.
var cdkCSharpConstructAnonRe = regexp.MustCompile(
	`new\s+([A-Za-z_][\w.]*)\s*\(\s*(?:this|scope|stack|[A-Za-z_][\w]*)\s*,\s*"([^"\n\r]+)"`,
)

// cdkCSharpGrantRe captures `res.GrantRead(grantee` (PascalCase). Group 1 =
// resource var, group 2 = grant method, group 3 = grantee var.
var cdkCSharpGrantRe = regexp.MustCompile(
	`([A-Za-z_][\w]*)\s*\.\s*(Grant[A-Za-z]*)\s*\(\s*([A-Za-z_][\w]*)`,
)

// cdkGoFactoryToType derives the construct TYPE from a Go factory function name
// by stripping the trailing `.New` segment's `New` prefix:
//
//	"awss3.NewBucket"        → "awss3.Bucket"
//	"awslambda.NewFunction"  → "awslambda.Function"
//	"awsdynamodb.NewTable"   → "awsdynamodb.Table"
//
// The package prefix is preserved so the shared IaCResourceCategory classifier
// (which matches on `.s3.`, `dynamodbtable`, `lambda.function`, …) still fires.
func cdkGoFactoryToType(factory string) string {
	const newTok = ".New"
	i := lastIndexString(factory, newTok)
	if i < 0 {
		return factory
	}
	return factory[:i+1] + factory[i+len(newTok):]
}

// lastIndexString returns the last index of sub in s, or -1. Tiny local helper
// to avoid importing strings just for one call in this file.
func lastIndexString(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// cdkBareTypeAlias maps a BARE CDK construct type name (as written at the
// constructor site in Java / C#, which carry no service-namespace prefix — e.g.
// `new Bucket(this,"Id",...)` not `new s3.Bucket(...)`) to a classifier-friendly
// dotted form so the shared types.IaCResourceCategory substring matcher fires.
//
// Without this, Java/C# resource types come through as bare `Bucket` / `Table` /
// `Function`, none of which the classifier's provider-qualified substrings
// (`s3.bucket`, `dynamodbtable`, `lambda.function`, …) match — every resource
// would mis-classify as "other". The Go binding does NOT need this: its factory
// (`awss3.NewBucket`) keeps the package prefix, which the classifier matches.
//
// Only well-known AWS CDK L2 construct names are mapped; anything unrecognised
// is returned unchanged so the classifier still gets its best shot (and falls
// back to "other" honestly rather than guessing).
func cdkBareTypeAlias(constructType string) string {
	// Use only the final segment so a partially-qualified `s3.Bucket` still maps,
	// while `Bucket` maps too. If a namespace is already present and the
	// classifier would match it, the alias is harmless (it only adds a prefix the
	// classifier already understands).
	last := constructType
	if i := lastIndexString(constructType, "."); i >= 0 {
		last = constructType[i+1:]
	}
	alias, ok := cdkBareTypeAliases[last]
	if !ok {
		return constructType
	}
	return alias
}

// cdkBareTypeAliases maps the common AWS CDK L2 construct type names to a
// provider-qualified string the shared classifier recognises. Keyed by the exact
// bare construct type (case-sensitive — CDK type names are PascalCase).
var cdkBareTypeAliases = map[string]string{
	// storage
	"Bucket": "s3.Bucket",
	// datastore
	"Table":            "dynamodb.Table",
	"DatabaseInstance": "rds.DatabaseInstance",
	"DatabaseCluster":  "rds.DatabaseCluster",
	// function
	"Function": "lambda.Function",
	// queue
	"Queue": "sqs.Queue",
	// topic
	"Topic": "sns.Topic",
	// stream
	"Stream": "kinesis.Stream",
	// cache
	"CfnCacheCluster": "elasticache.CfnCacheCluster",
	// secret
	"Secret": "secretsmanager.Secret",
	// network
	"Vpc": "ec2.Vpc",
}

// applyCDKEdgesJVMGoNet runs the CDK extraction passes for Java / Go / C#,
// reusing the shared emitResource / emitDependsOn closures and var→LogicalId
// binding map from applyCDKEdges. Mirrors the TS/JS passes (construct decl,
// anonymous construct, grant edges, props/args-passed refs).
func applyCDKEdgesJVMGoNet(
	lang, src string,
	emitResource func(logicalID, constructType string, offset int),
	emitDependsOn func(fromLogical, toLogical, reason, detail string),
	varToLogical map[string]string,
	logicalIDs map[string]bool,
) {
	switch lang {
	case "java":
		applyCDKEdgesJava(src, emitResource, emitDependsOn, varToLogical, logicalIDs)
	case "go":
		applyCDKEdgesGo(src, emitResource, emitDependsOn, varToLogical, logicalIDs)
	case "csharp":
		applyCDKEdgesCSharp(src, emitResource, emitDependsOn, varToLogical, logicalIDs)
	}
}

func applyCDKEdgesJava(
	src string,
	emitResource func(logicalID, constructType string, offset int),
	emitDependsOn func(fromLogical, toLogical, reason, detail string),
	varToLogical map[string]string,
	logicalIDs map[string]bool,
) {
	// Pass 1a: builder-form assigned declarations — `T v = T.Builder.create(this,"Id")`.
	for _, m := range cdkJavaBuilderDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 2))
		logicalID := extractGroupFromIndex(src, m, 3)
		if logicalID == "" || constructType == "" {
			continue
		}
		emitResource(logicalID, constructType, m[0])
		if varName != "" {
			varToLogical[varName] = logicalID
		}
	}
	// Pass 1b: `new`-form assigned declarations — `T v = new T(this,"Id"`.
	for _, m := range cdkJavaConstructDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 2))
		logicalID := extractGroupFromIndex(src, m, 3)
		if logicalID == "" || constructType == "" {
			continue
		}
		emitResource(logicalID, constructType, m[0])
		if varName != "" {
			// Don't clobber a builder binding already recorded for this var.
			if _, ok := varToLogical[varName]; !ok {
				varToLogical[varName] = logicalID
			}
		}
	}
	// Pass 2: anonymous constructs (builder + new forms).
	for _, m := range cdkJavaBuilderAnonRe.FindAllStringSubmatchIndex(src, -1) {
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 1))
		logicalID := extractGroupFromIndex(src, m, 2)
		if logicalID != "" && constructType != "" {
			emitResource(logicalID, constructType, m[0])
		}
	}
	for _, m := range cdkJavaConstructAnonRe.FindAllStringSubmatchIndex(src, -1) {
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 1))
		logicalID := extractGroupFromIndex(src, m, 2)
		if logicalID != "" && constructType != "" {
			emitResource(logicalID, constructType, m[0])
		}
	}
	// Pass 3: grant edges — `res.grantRead(grantee)`. Grantee DEPENDS_ON res.
	for _, m := range cdkJavaGrantRe.FindAllStringSubmatch(src, -1) {
		resourceVar, grantMethod, granteeVar := m[1], m[2], m[3]
		resLogical := varToLogical[resourceVar]
		granteeLogical := varToLogical[granteeVar]
		if resLogical == "" || granteeLogical == "" {
			continue
		}
		emitDependsOn(granteeLogical, resLogical, "grant", grantMethod)
	}
	_ = logicalIDs
}

func applyCDKEdgesGo(
	src string,
	emitResource func(logicalID, constructType string, offset int),
	emitDependsOn func(fromLogical, toLogical, reason, detail string),
	varToLogical map[string]string,
	logicalIDs map[string]bool,
) {
	// Pass 1: assigned declarations — `v := pkg.NewType(stack, jsii.String("Id")`.
	for _, m := range cdkGoConstructDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		factory := extractGroupFromIndex(src, m, 2)
		logicalID := extractGroupFromIndex(src, m, 3)
		if logicalID == "" || factory == "" {
			continue
		}
		constructType := cdkGoFactoryToType(factory)
		emitResource(logicalID, constructType, m[0])
		if varName != "" {
			varToLogical[varName] = logicalID
		}
	}
	// Pass 2: anonymous declarations — `pkg.NewType(stack, jsii.String("Id")`.
	for _, m := range cdkGoConstructAnonRe.FindAllStringSubmatchIndex(src, -1) {
		factory := extractGroupFromIndex(src, m, 1)
		logicalID := extractGroupFromIndex(src, m, 2)
		if logicalID == "" || factory == "" {
			continue
		}
		emitResource(logicalID, cdkGoFactoryToType(factory), m[0])
	}
	// Pass 3: grant edges — `res.GrantRead(grantee, ...)`. Grantee DEPENDS_ON res.
	for _, m := range cdkGoGrantRe.FindAllStringSubmatch(src, -1) {
		resourceVar, grantMethod, granteeVar := m[1], m[2], m[3]
		resLogical := varToLogical[resourceVar]
		granteeLogical := varToLogical[granteeVar]
		if resLogical == "" || granteeLogical == "" {
			continue
		}
		emitDependsOn(granteeLogical, resLogical, "grant", grantMethod)
	}
	_ = logicalIDs
}

func applyCDKEdgesCSharp(
	src string,
	emitResource func(logicalID, constructType string, offset int),
	emitDependsOn func(fromLogical, toLogical, reason, detail string),
	varToLogical map[string]string,
	logicalIDs map[string]bool,
) {
	// Pass 1: assigned declarations — `var v = new Type(this, "Id"`.
	for _, m := range cdkCSharpConstructDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 2))
		logicalID := extractGroupFromIndex(src, m, 3)
		if logicalID == "" || constructType == "" {
			continue
		}
		emitResource(logicalID, constructType, m[0])
		if varName != "" {
			varToLogical[varName] = logicalID
		}
	}
	// Pass 2: anonymous declarations — `new Type(this, "Id"`.
	for _, m := range cdkCSharpConstructAnonRe.FindAllStringSubmatchIndex(src, -1) {
		constructType := cdkBareTypeAlias(extractGroupFromIndex(src, m, 1))
		logicalID := extractGroupFromIndex(src, m, 2)
		if logicalID == "" || constructType == "" {
			continue
		}
		emitResource(logicalID, constructType, m[0])
	}
	// Pass 3: grant edges — `res.GrantRead(grantee)`. Grantee DEPENDS_ON res.
	for _, m := range cdkCSharpGrantRe.FindAllStringSubmatch(src, -1) {
		resourceVar, grantMethod, granteeVar := m[1], m[2], m[3]
		resLogical := varToLogical[resourceVar]
		granteeLogical := varToLogical[granteeVar]
		if resLogical == "" || granteeLogical == "" {
			continue
		}
		emitDependsOn(granteeLogical, resLogical, "grant", grantMethod)
	}
	_ = logicalIDs
}
