// Pulumi — Go / .NET (C#) language-binding resource + dependency extraction.
// Extends pulumi_edges.go (which covers Pulumi-TS/JS + Pulumi-Python) to the Go
// and C# Pulumi SDKs. Part of #3550, epic #3512.
//
// Background / the gap this closes
// --------------------------------
// Pulumi is multi-language: a resource is a constructor call whose FIRST string
// argument is the resource's logical name, in every SDK. pulumi_edges.go covered
// TS/JS + Python; the registry honestly stamped Pulumi-Go/C#/Java "not yet
// implemented". This pass closes Go and C# (the two remaining widely-used Pulumi
// SDKs with a stable single-file constructor idiom), emitting the same
// SCOPE.InfraResource entities + DEPENDS_ON edges + resource categories as the
// TS path so cross-language IaC queries are uniform.
//
// What this pass extracts (per binding)
// -------------------------------------
//
//	Go   bucket, err := s3.NewBucket(ctx, "assets", &s3.BucketArgs{...})
//	     policy, err := iam.NewBucketPolicy(ctx, "policy", &iam.BucketPolicyArgs{
//	         Bucket: bucket.ID(),                 // output ref → DEPENDS_ON
//	     }, pulumi.DependsOn([]pulumi.Resource{bucket}))   // explicit dep
//	C#   var bucket = new Aws.S3.Bucket("assets", new BucketArgs { ... });
//	     var policy = new Aws.S3.BucketPolicy("policy", new BucketPolicyArgs {
//	         Bucket = bucket.Id,
//	     }, new CustomResourceOptions { DependsOn = { bucket } });
//
// For each resource we emit a SCOPE.InfraResource NAMED by its logical-name
// string literal, with construct_type + the uniform resource_category from the
// shared types.IaCResourceCategory classifier (#3549). Dependency edges
// (consumer DEPENDS_ON producer) come from output references (`producer.Id` /
// `.Arn` / `.Bucket` passed into another resource's args) and explicit
// DependsOn lists.
//
// Honest scope (partial)
// ----------------------
// Single-file regex passes mirroring the TS path. The Go factory→type mapping
// strips `New` from `pkg.NewType`. Cross-file references, ComponentResource
// subclasses, and StackReference handling for these two SDKs are deferred to a
// follow-up; the registry stamps these bindings `partial`. Append-only.
//
// Refs #3550, #3512.
package engine

import "regexp"

// pulumiSupportsLanguageGoNet reports whether this file handles `lang`.
func pulumiSupportsLanguageGoNet(lang string) bool {
	return lang == "go" || lang == "csharp"
}

// ---------------------------------------------------------------------------
// Go idioms.
//
//	bucket, err := s3.NewBucket(ctx, "assets", &s3.BucketArgs{...})
//	policy, err := iam.NewBucketPolicy(ctx, "policy", &iam.BucketPolicyArgs{
//	    Bucket: bucket.ID(),
//	}, pulumi.DependsOn([]pulumi.Resource{bucket}))
// ---------------------------------------------------------------------------

// pulumiGoResourceDeclRe captures `var[, err] := pkg.NewType(ctx, "name"`.
// Group 1 = the Go var (resource handle), group 2 = the package-qualified
// factory (e.g. `s3.NewBucket`), group 3 = the "name" logical-name literal. A
// trailing `, err` before `:=` is tolerated by the optional `(?:\s*,\s*\w+)?`.
var pulumiGoResourceDeclRe = regexp.MustCompile(
	`([A-Za-z_][\w]*)\s*(?:,\s*[A-Za-z_][\w]*\s*)?:?=\s*([A-Za-z_][\w.]*\.New[A-Za-z_][\w]*)\s*\(\s*(?:ctx|[A-Za-z_][\w]*)\s*,\s*"([^"\n\r]+)"`,
)

// pulumiGoResourceAnonRe captures an UNASSIGNED `pkg.NewType(ctx, "name"`.
// Group 1 = factory, group 2 = logical name.
var pulumiGoResourceAnonRe = regexp.MustCompile(
	`([A-Za-z_][\w.]*\.New[A-Za-z_][\w]*)\s*\(\s*(?:ctx|[A-Za-z_][\w]*)\s*,\s*"([^"\n\r]+)"`,
)

// pulumiGoResourceArgsRe matches a full Go resource constructor and captures its
// args body (everything after the logical-name literal up to the call's closing
// `)`), so it can be scanned for output refs and DependsOn lists. Group 1 =
// logical name, group 2 = args body.
var pulumiGoResourceArgsRe = regexp.MustCompile(
	`[A-Za-z_][\w.]*\.New[A-Za-z_][\w]*\s*\(\s*(?:ctx|[A-Za-z_][\w]*)\s*,\s*"([^"\n\r]+)"\s*,([\s\S]{0,800}?)\)\n`,
)

// pulumiGoOutputRefRe finds `producer.Id(` / `.Arn` / `.ID()` / `.URL` output
// references inside a Go args body. Group 1 = the producing resource var. Go
// outputs are PascalCase accessors (`.Arn`, `.ID()`, `.Bucket`).
var pulumiGoOutputRefRe = regexp.MustCompile(
	`\b([A-Za-z_][\w]*)\s*\.\s*(?:Arn|ID|Id|URL|Url|Name|Endpoint|Bucket|TopicArn|QueueUrl)\b`,
)

// pulumiGoDependsOnRe captures a `pulumi.DependsOn([]pulumi.Resource{ ... })`
// list body. Group 1 = the raw `{...}` contents (resource vars).
var pulumiGoDependsOnRe = regexp.MustCompile(
	`DependsOn\s*\(\s*\[\]pulumi\.Resource\s*\{([^}]*)\}`,
)

// ---------------------------------------------------------------------------
// C# idioms.
//
//	var bucket = new Aws.S3.Bucket("assets", new BucketArgs { ... });
//	var policy = new Aws.S3.BucketPolicy("policy", new BucketPolicyArgs {
//	    Bucket = bucket.Id,
//	}, new CustomResourceOptions { DependsOn = { bucket } });
// ---------------------------------------------------------------------------

// pulumiCSharpResourceDeclRe captures `[Type ]var = new Provider.Svc.Type("name"`.
// Group 1 = var, group 2 = resource type, group 3 = logical name.
var pulumiCSharpResourceDeclRe = regexp.MustCompile(
	`(?:[A-Za-z_][\w.<>]*\s+)?([A-Za-z_][\w]*)\s*=\s*new\s+([A-Za-z_][\w.]*)\s*\(\s*"([^"\n\r]+)"`,
)

// pulumiCSharpResourceAnonRe captures an UNASSIGNED `new Provider.Svc.Type("name"`.
// Group 1 = resource type, group 2 = logical name.
var pulumiCSharpResourceAnonRe = regexp.MustCompile(
	`new\s+([A-Za-z_][\w.]*)\s*\(\s*"([^"\n\r]+)"`,
)

// pulumiCSharpResourceArgsRe matches a full C# resource constructor and captures
// its args body. Group 1 = logical name, group 2 = args body.
var pulumiCSharpResourceArgsRe = regexp.MustCompile(
	`new\s+[A-Za-z_][\w.]*\s*\(\s*"([^"\n\r]+)"\s*,([\s\S]{0,800}?)\)\s*;`,
)

// pulumiCSharpOutputRefRe finds `producer.Id` / `.Arn` / `.Url` output refs in a
// C# args body. Group 1 = the producing resource var.
var pulumiCSharpOutputRefRe = regexp.MustCompile(
	`\b([A-Za-z_][\w]*)\s*\.\s*(?:Arn|Id|Url|Name|Endpoint|Bucket|TopicArn|QueueUrl)\b`,
)

// pulumiCSharpDependsOnRe captures a `DependsOn = { ... }` list body. Group 1 =
// the raw `{...}` contents (resource vars).
var pulumiCSharpDependsOnRe = regexp.MustCompile(
	`DependsOn\s*=\s*\{([^}]*)\}`,
)

// pulumiGoNetIdentRe extracts bare identifiers from a dependsOn list body.
var pulumiGoNetIdentRe = regexp.MustCompile(`[A-Za-z_][\w]*`)

// applyPulumiEdgesGoNet runs the Pulumi extraction passes for Go / C#, reusing
// the shared emit closures and var→name binding map from applyPulumiEdges.
func applyPulumiEdgesGoNet(
	lang, src string,
	emitResource func(logicalName, resourceType, scopeOverride string, offset int),
	emitDependsOn func(fromName, toName, reason, detail string),
	varToName map[string]string,
	resourceNames map[string]bool,
) {
	switch lang {
	case "go":
		applyPulumiEdgesGo(src, emitResource, emitDependsOn, varToName, resourceNames)
	case "csharp":
		applyPulumiEdgesCSharp(src, emitResource, emitDependsOn, varToName, resourceNames)
	}
}

func applyPulumiEdgesGo(
	src string,
	emitResource func(logicalName, resourceType, scopeOverride string, offset int),
	emitDependsOn func(fromName, toName, reason, detail string),
	varToName map[string]string,
	resourceNames map[string]bool,
) {
	// Pass 1: assigned resource declarations — `v, err := pkg.NewType(ctx,"name"`.
	for _, m := range pulumiGoResourceDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		factory := extractGroupFromIndex(src, m, 2)
		logicalName := extractGroupFromIndex(src, m, 3)
		if logicalName == "" || factory == "" {
			continue
		}
		resourceType := cdkGoFactoryToType(factory) // same New-strip mapping
		emitResource(logicalName, resourceType, "", m[0])
		if varName != "" {
			varToName[varName] = logicalName
		}
	}
	// Pass 1b: anonymous resource instantiations — `pkg.NewType(ctx, "name"`.
	for _, m := range pulumiGoResourceAnonRe.FindAllStringSubmatchIndex(src, -1) {
		factory := extractGroupFromIndex(src, m, 1)
		logicalName := extractGroupFromIndex(src, m, 2)
		if logicalName == "" || factory == "" {
			continue
		}
		emitResource(logicalName, cdkGoFactoryToType(factory), "", m[0])
	}
	// Pass 2: dependency edges from args — output refs + DependsOn lists.
	for _, m := range pulumiGoResourceArgsRe.FindAllStringSubmatch(src, -1) {
		consumerName := m[1]
		argsBody := m[2]
		if !resourceNames[consumerName] || argsBody == "" {
			continue
		}
		for _, rm := range pulumiGoOutputRefRe.FindAllStringSubmatch(argsBody, -1) {
			producerVar := rm[1]
			producerName, ok := varToName[producerVar]
			if !ok || producerName == consumerName {
				continue
			}
			emitDependsOn(consumerName, producerName, "output_ref", producerVar)
		}
		for _, dm := range pulumiGoDependsOnRe.FindAllStringSubmatch(argsBody, -1) {
			for _, im := range pulumiGoNetIdentRe.FindAllString(dm[1], -1) {
				depName, ok := varToName[im]
				if !ok || depName == consumerName {
					continue
				}
				emitDependsOn(consumerName, depName, "depends_on", im)
			}
		}
	}
}

func applyPulumiEdgesCSharp(
	src string,
	emitResource func(logicalName, resourceType, scopeOverride string, offset int),
	emitDependsOn func(fromName, toName, reason, detail string),
	varToName map[string]string,
	resourceNames map[string]bool,
) {
	// Pass 1: assigned resource declarations — `var v = new Provider.Svc.Type("name"`.
	for _, m := range pulumiCSharpResourceDeclRe.FindAllStringSubmatchIndex(src, -1) {
		varName := extractGroupFromIndex(src, m, 1)
		resourceType := extractGroupFromIndex(src, m, 2)
		logicalName := extractGroupFromIndex(src, m, 3)
		if logicalName == "" || resourceType == "" {
			continue
		}
		emitResource(logicalName, resourceType, "", m[0])
		if varName != "" {
			varToName[varName] = logicalName
		}
	}
	// Pass 1b: anonymous resource instantiations — `new Provider.Svc.Type("name"`.
	for _, m := range pulumiCSharpResourceAnonRe.FindAllStringSubmatchIndex(src, -1) {
		resourceType := extractGroupFromIndex(src, m, 1)
		logicalName := extractGroupFromIndex(src, m, 2)
		if logicalName == "" || resourceType == "" {
			continue
		}
		emitResource(logicalName, resourceType, "", m[0])
	}
	// Pass 2: dependency edges from args — output refs + DependsOn lists.
	for _, m := range pulumiCSharpResourceArgsRe.FindAllStringSubmatch(src, -1) {
		consumerName := m[1]
		argsBody := m[2]
		if !resourceNames[consumerName] || argsBody == "" {
			continue
		}
		for _, rm := range pulumiCSharpOutputRefRe.FindAllStringSubmatch(argsBody, -1) {
			producerVar := rm[1]
			producerName, ok := varToName[producerVar]
			if !ok || producerName == consumerName {
				continue
			}
			emitDependsOn(consumerName, producerName, "output_ref", producerVar)
		}
		for _, dm := range pulumiCSharpDependsOnRe.FindAllStringSubmatch(argsBody, -1) {
			for _, im := range pulumiGoNetIdentRe.FindAllString(dm[1], -1) {
				depName, ok := varToName[im]
				if !ok || depName == consumerName {
					continue
				}
				emitDependsOn(consumerName, depName, "depends_on", im)
			}
		}
	}
}
