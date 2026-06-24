package dashboard

import "testing"

func TestIaCToolForEntity(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		subtype  string
		language string
		props    map[string]string
		wantTool string
		wantOK   bool
	}{
		{"cdk infraresource", "SCOPE.InfraResource", "", "typescript",
			map[string]string{"iac_tool": "aws-cdk"}, "aws-cdk", true},
		{"pulumi", "SCOPE.InfraResource", "", "python",
			map[string]string{"iac_tool": "pulumi"}, "pulumi", true},
		{"cfn datastore via semantic kind", "SCOPE.Datastore", "", "yaml",
			map[string]string{"iac_tool": "cloudformation"}, "cloudformation", true},
		{"terraform resource (no iac_tool prop)", "SCOPE.Component", "resource", "terraform",
			nil, "terraform", true},
		{"hcl resource alias", "SCOPE.Component", "resource", "hcl",
			nil, "terraform", true},
		{"terraform module block renders (#4625)", "SCOPE.Component", "module", "terraform",
			nil, "terraform", true},
		{"hcl module block renders (#4625)", "SCOPE.Component", "module", "hcl",
			nil, "terraform", true},
		{"terraform provider block is not a resource", "SCOPE.Component", "provider", "terraform",
			nil, "", false},
		{"plain component (non-iac)", "SCOPE.Component", "resource", "go",
			nil, "", false},
		{"non-iac function", "SCOPE.Function", "", "go", nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool, ok := iacToolForEntity(c.kind, c.subtype, c.language, c.props)
			if tool != c.wantTool || ok != c.wantOK {
				t.Fatalf("iacToolForEntity = (%q,%v), want (%q,%v)", tool, ok, c.wantTool, c.wantOK)
			}
		})
	}
}

func TestIaCResourceTypeOf(t *testing.T) {
	cases := []struct {
		name  string
		ename string
		props map[string]string
		want  string
	}{
		{"cdk construct_type", "DataBucket", map[string]string{"construct_type": "s3.Bucket"}, "s3.Bucket"},
		{"cfn resource_type", "MyTable", map[string]string{"resource_type": "AWS::DynamoDB::Table"}, "AWS::DynamoDB::Table"},
		{"terraform name-encoded", "aws_db_instance.main", nil, "aws_db_instance"},
		{"terraform module (#4625)", "module.dispatch_queue", nil, "module"},
		{"no type", "thing", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := iacResourceTypeOf(c.ename, c.props); got != c.want {
				t.Fatalf("iacResourceTypeOf = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIaCCategoryOf(t *testing.T) {
	// Explicit property wins.
	if got := iacCategoryOf("s3.Bucket", map[string]string{"resource_category": "storage"}); got != "storage" {
		t.Fatalf("explicit category = %q, want storage", got)
	}
	// resource_scope back-compat fallback.
	if got := iacCategoryOf("", map[string]string{"resource_scope": "queue"}); got != "queue" {
		t.Fatalf("resource_scope fallback = %q, want queue", got)
	}
	// Recomputed from type (Terraform path — category not in Properties).
	if got := iacCategoryOf("aws_db_instance", nil); got != "datastore" {
		t.Fatalf("recomputed category = %q, want datastore", got)
	}
	if got := iacCategoryOf("", nil); got != "" {
		t.Fatalf("empty category = %q, want empty", got)
	}
	// #4625 — a Terraform module instance is its own diagram category.
	if got := iacCategoryOf("module", nil); got != "module" {
		t.Fatalf("module category = %q, want module", got)
	}
}

func TestIaCRelationFacet(t *testing.T) {
	cases := []struct {
		name       string
		kind       string
		props      map[string]string
		wantFacet  string
		wantDetail string
	}{
		{"grant", "DEPENDS_ON", map[string]string{"reason": "grant", "grant": "grantReadWrite"}, "grant", "grantReadWrite"},
		{"event_source", "DEPENDS_ON", map[string]string{"reason": "event_source"}, "event_source", ""},
		{"props_ref dependency", "DEPENDS_ON", map[string]string{"reason": "props_ref", "props_ref": "dataBucket"}, "dependency", "dataBucket"},
		{"plain depends_on", "DEPENDS_ON", nil, "dependency", ""},
		{"contains topology", "CONTAINS", nil, "topology", ""},
		{"sam trigger", "TRIGGERS", map[string]string{"trigger": "Api"}, "trigger", "Api"},
		{"serves route", "SERVES", map[string]string{"http_method": "GET", "route_path": "/x"}, "trigger", "GET"},
		// #4625 — cross-module output ref carries a derived semantic verb as the facet.
		{"cross-module consumes", "USES",
			map[string]string{"dataflow": "cross_module", "semantic": "consumes", "module_output": "queue_url"},
			"consumes", "queue_url"},
		{"cross-module redrive", "USES",
			map[string]string{"dataflow": "cross_module", "semantic": "redrive", "module_output": "queue_arn"},
			"redrive", "queue_arn"},
		{"cross-module generic falls back to dependency", "USES",
			map[string]string{"dataflow": "cross_module", "semantic": "dependency", "module_output": "id"},
			"dependency", "id"},
		// #4657 — module instantiation edge surfaces as its own facet.
		{"instantiates", "INSTANTIATES",
			map[string]string{"definition_dir": "modules/worker-service"},
			"instantiates", "modules/worker-service"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			facet, detail := iacRelationFacet(c.kind, c.props)
			if facet != c.wantFacet || detail != c.wantDetail {
				t.Fatalf("iacRelationFacet = (%q,%q), want (%q,%q)", facet, detail, c.wantFacet, c.wantDetail)
			}
		})
	}
}

func TestIaCIsOutputEntity(t *testing.T) {
	if !iacIsOutputEntity("SCOPE.Config", "", map[string]string{"export_name": "BucketArn"}) {
		t.Fatal("cfn export should be an output entity")
	}
	if !iacIsOutputEntity("SCOPE.Schema", "output", nil) {
		t.Fatal("hcl output should be an output entity")
	}
	if iacIsOutputEntity("SCOPE.InfraResource", "", map[string]string{"iac_tool": "aws-cdk"}) {
		t.Fatal("a resource is not an output entity")
	}
}

func TestMergeEnv(t *testing.T) {
	cases := []struct {
		existing, add, want string
	}{
		{"", "prod", "prod"},
		{"prod", "prod", "prod"},
		{"prod", "dev", "dev,prod"},
		{"dev,prod", "staging", "dev,prod,staging"},
		{"prod", "", "prod"},
	}
	for _, c := range cases {
		if got := mergeEnv(c.existing, c.add); got != c.want {
			t.Errorf("mergeEnv(%q,%q) = %q, want %q", c.existing, c.add, got, c.want)
		}
	}
}

func TestSplitEnv(t *testing.T) {
	got := splitEnv(" dev , prod ,")
	if len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Errorf("splitEnv = %v, want [dev prod]", got)
	}
	if splitEnv("") != nil {
		t.Error("splitEnv(\"\") should be nil")
	}
}

// TestJoinModuleInstantiations_ContainmentAndEnv covers #4862 + #4657: each
// definition resource gets ParentID = its instantiating module instance, env is
// propagated onto the (env-less) definition, and INSTANTIATES relations are
// drawn both directions. The FIRST instance (by entity id) wins containment.
func TestJoinModuleInstantiations_ContainmentAndEnv(t *testing.T) {
	// Two env instances of the same worker-service definition.
	prod := &IaCResource{
		EntityID:      "infra/inst-prod",
		Repo:          "infra",
		Name:          "module.worker_prod",
		DefinitionDir: "modules/worker-service",
		Env:           "prod",
	}
	dev := &IaCResource{
		EntityID:      "infra/inst-dev",
		Repo:          "infra",
		Name:          "module.worker_dev",
		DefinitionDir: "modules/worker-service",
		Env:           "dev",
	}
	// Definition resources live under the definition directory (Module == dir).
	task := &IaCResource{
		EntityID: "infra/def-task",
		Repo:     "infra",
		Name:     "aws_ecs_task_definition.worker",
		Module:   "modules/worker-service",
	}
	queue := &IaCResource{
		EntityID: "infra/def-queue",
		Repo:     "infra",
		Name:     "aws_sqs_queue.work",
		Module:   "modules/worker-service",
	}

	byID := map[string]*IaCResource{
		prod.EntityID:  prod,
		dev.EntityID:   dev,
		task.EntityID:  task,
		queue.EntityID: queue,
	}

	joinModuleInstantiations(byID, "infra")

	// #4884 — a definition instantiated across MORE THAN ONE env (dev + prod) is
	// CROSS-ENV: a single ParentID cannot serve both env tabs (the diagram is
	// env-scoped and each env renders only its own instances), so ParentID is left
	// EMPTY and the frontend nests it per-env from each rendered instance's
	// instantiates edges. (Before #4884 it locked to the first instance by entity
	// id — "infra/inst-dev" — which the prod tab then filtered out, leaving the
	// resource un-nested in its definition zone with the fan-out edge intact.)
	if task.ParentID != "" {
		t.Fatalf("task.ParentID = %q, want empty for cross-env def", task.ParentID)
	}
	if queue.ParentID != "" {
		t.Fatalf("queue.ParentID = %q, want empty for cross-env def", queue.ParentID)
	}

	// Env propagated from BOTH instantiating envs onto the shared definitions.
	if task.Env != "dev,prod" {
		t.Fatalf("task.Env = %q, want dev,prod", task.Env)
	}

	// Each instance gained an outbound INSTANTIATES relation per definition
	// resource; each definition gained an inbound one per instance.
	countFacet := func(r *IaCResource, dir string) int {
		n := 0
		for _, rel := range r.Relations {
			if rel.Facet == "instantiates" && rel.Direction == dir {
				n++
			}
		}
		return n
	}
	if got := countFacet(dev, "out"); got != 2 {
		t.Fatalf("dev out instantiates = %d, want 2", got)
	}
	if got := countFacet(task, "in"); got != 2 { // from dev + prod
		t.Fatalf("task in instantiates = %d, want 2", got)
	}

	// An instance does not instantiate itself, and a resource with no
	// instantiation keeps an empty ParentID.
	if dev.ParentID != "" {
		t.Fatalf("instance dev.ParentID = %q, want empty", dev.ParentID)
	}

	// #4884 — each env instance keeps an OUTBOUND instantiates edge to every def
	// it instantiates, so the frontend can resolve containment per-env even when
	// ParentID is empty. On the prod tab, only the prod instance + the (env=prod-
	// inclusive) defs render; prod's instantiates edges name task + queue.
	if got := countFacet(prod, "out"); got != 2 {
		t.Fatalf("prod out instantiates = %d, want 2", got)
	}
	prodTargets := map[string]bool{}
	for _, rel := range prod.Relations {
		if rel.Facet == "instantiates" && rel.Direction == "out" {
			prodTargets[rel.TargetEntityID] = true
		}
	}
	if !prodTargets[task.EntityID] || !prodTargets[queue.EntityID] {
		t.Fatalf("prod instantiates targets = %v, want both %q and %q",
			prodTargets, task.EntityID, queue.EntityID)
	}
}

// TestJoinModuleInstantiations_SingleEnvKeepsParentID is the #4884 control: a
// definition instantiated within a SINGLE env DOES keep a ParentID (the parent
// instance is guaranteed to render alongside it on that env's tab), so the
// simple one-env stack still nests via the direct parent_id path. This mirrors
// the live acme-v3 shape where the directory-join (resolveModuleSourceDir →
// iacModuleOf) produces matching dirs (e.g. infra/terraform/modules/worker-
// service) so the join fires; the regression was purely the cross-env ParentID
// pointing at a filtered-out instance.
func TestJoinModuleInstantiations_SingleEnvKeepsParentID(t *testing.T) {
	// Module dir as iacModuleOf would derive it for a real acme-v3 def file
	// infra/terraform/modules/worker-service/main.tf, and the matching
	// definition_dir resolveModuleSourceDir derives from
	// infra/terraform/envs/prod/main.tf + source ../../modules/worker-service.
	const defDir = "infra/terraform/modules/worker-service"
	prod := &IaCResource{
		EntityID:      "infra/inst-prod",
		Repo:          "infra",
		Name:          "module.worker",
		DefinitionDir: defDir,
		Env:           "prod",
	}
	task := &IaCResource{
		EntityID: "infra/def-task",
		Repo:     "infra",
		Name:     "aws_ecs_task_definition.worker",
		Module:   defDir,
	}
	byID := map[string]*IaCResource{
		prod.EntityID: prod,
		task.EntityID: task,
	}

	joinModuleInstantiations(byID, "infra")

	if task.ParentID != prod.EntityID {
		t.Fatalf("single-env task.ParentID = %q, want %q", task.ParentID, prod.EntityID)
	}
	if task.Env != "prod" {
		t.Fatalf("single-env task.Env = %q, want prod", task.Env)
	}
}

func TestIDTail(t *testing.T) {
	if got := idTail("SCOPE.InfraResource:DataBucket"); got != "DataBucket" {
		t.Fatalf("idTail = %q, want DataBucket", got)
	}
	if got := idTail("noColon"); got != "noColon" {
		t.Fatalf("idTail = %q, want noColon", got)
	}
}

// TestIaCCanonicalResourceRef covers the #4864 recovery of a canonical
// Terraform resource reference from an unresolved relation endpoint id.
func TestIaCCanonicalResourceRef(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Structural-ref form the HCL extractor emits for a cross-file ref.
		{"structural-ref resource", "scope:operation:method:terraform:infra/iam/main.tf:aws_iam_role.execution", "aws_iam_role.execution"},
		// Bare canonical form.
		{"bare resource", "aws_sqs_queue.work", "aws_sqs_queue.work"},
		// Attribute interpolation collapses to the OWNING resource (#4864 case b).
		{"interpolation attr", "scope:operation:method:terraform:infra/iam/main.tf:aws_iam_role.execution.arn", "aws_iam_role.execution"},
		{"bare interpolation attr", "aws_cloudwatch_log_group.ecs.name", "aws_cloudwatch_log_group.ecs"},
		// Non-resource heads stay chips (return "").
		{"var", "scope:operation:method:terraform:x.tf:var.region", ""},
		{"local", "local.tags", ""},
		{"output", "output.url", ""},
		{"data", "data.aws_ami.ubuntu", ""},
		{"provider", "provider.aws", ""},
		{"module", "module.network", ""},
		{"path", "path.module", ""},
		{"terraform", "terraform.workspace", ""},
		// Not a two-segment dotted ref.
		{"single segment", "scope:operation:method:terraform:x.tf:count", ""},
		{"empty", "", ""},
		{"trailing dot", "aws_iam_role.", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := iacCanonicalResourceRef(tc.in); got != tc.want {
				t.Fatalf("iacCanonicalResourceRef(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestAttachIaCRelation_SecondaryNameJoin is the #4864 regression: a resource
// referencing another resource via a CROSS-FILE structural-ref that the global
// resolver could not bind (byLocation is file-scoped) must still become a
// resolved resource→resource edge (TargetEntityID set), NOT an unresolved chip.
// A reference to a non-resource (a variable) legitimately stays a chip.
func TestAttachIaCRelation_SecondaryNameJoin(t *testing.T) {
	// Two rendered resources in the same repo, defined in different .tf files.
	task := &IaCResource{
		EntityID:   "infra/ent-task",
		Repo:       "infra",
		Name:       "aws_ecs_task_definition.worker",
		SourceFile: "infra/ecs/main.tf",
		Relations:  []IaCRelation{},
	}
	role := &IaCResource{
		EntityID:   "infra/ent-role",
		Repo:       "infra",
		Name:       "aws_iam_role.execution",
		SourceFile: "infra/iam/main.tf",
		Relations:  []IaCRelation{},
	}

	byID := map[string]*IaCResource{
		"ent-task": task,
		"ent-role": role,
	}
	nameByID := map[string]string{
		"ent-task": task.Name,
		"ent-role": role.Name,
	}
	resByCanonRef := map[string]*IaCResource{
		iacCanonicalResourceRef(task.Name): task,
		iacCanonicalResourceRef(role.Name): role,
	}

	report := IaCReport{}

	// The ECS task references the IAM role via an ATTRIBUTE interpolation in a
	// SIBLING file. The global resolver left the edge with FromID = the task's
	// own block ref (resolved → ent-task) and ToID = the unbound cross-file
	// structural-ref encoding `aws_iam_role.execution.arn`.
	attachIaCRelation(
		&report, byID, nameByID, resByCanonRef,
		"ent-task",
		"scope:operation:method:terraform:infra/iam/main.tf:aws_iam_role.execution.arn",
		"USES", nil,
	)

	// The task's outbound relation must now resolve to the role node.
	if len(task.Relations) != 1 {
		t.Fatalf("task relations = %d, want 1", len(task.Relations))
	}
	out := task.Relations[0]
	if out.TargetEntityID != role.EntityID {
		t.Fatalf("out.TargetEntityID = %q, want %q (edge would render as an unresolved chip otherwise)", out.TargetEntityID, role.EntityID)
	}
	if !out.TargetResolved {
		t.Fatalf("out.TargetResolved = false, want true")
	}
	if out.Target != role.Name {
		t.Fatalf("out.Target = %q, want %q", out.Target, role.Name)
	}
	// The role gains the mirror inbound relation joined back to the task.
	if len(role.Relations) != 1 {
		t.Fatalf("role relations = %d, want 1", len(role.Relations))
	}
	if role.Relations[0].TargetEntityID != task.EntityID {
		t.Fatalf("role in.TargetEntityID = %q, want %q", role.Relations[0].TargetEntityID, task.EntityID)
	}

	// A reference to a VARIABLE legitimately stays an unresolved chip: the task
	// gets a relation with no TargetEntityID (the footer counts it).
	attachIaCRelation(
		&report, byID, nameByID, resByCanonRef,
		"ent-task",
		"scope:operation:method:terraform:infra/ecs/main.tf:var.cpu",
		"USES", nil,
	)
	if len(task.Relations) != 2 {
		t.Fatalf("task relations after var ref = %d, want 2", len(task.Relations))
	}
	varRel := task.Relations[1]
	if varRel.TargetEntityID != "" {
		t.Fatalf("var ref TargetEntityID = %q, want empty (must stay a chip)", varRel.TargetEntityID)
	}
}

// TestAttachIaCRelation_AmbiguousRefStaysChip verifies that when two resources
// in one repo share a canonical ref (dropped to nil in the index), the
// name-join does NOT fire — the edge stays an unresolved chip rather than
// binding to an arbitrary node.
func TestAttachIaCRelation_AmbiguousRefStaysChip(t *testing.T) {
	src := &IaCResource{EntityID: "infra/ent-src", Repo: "infra", Name: "aws_s3_bucket.logs", Relations: []IaCRelation{}}
	byID := map[string]*IaCResource{"ent-src": src}
	nameByID := map[string]string{"ent-src": src.Name}
	// Ambiguous target ref was dropped to nil during indexing.
	resByCanonRef := map[string]*IaCResource{"aws_iam_role.shared": nil}
	report := IaCReport{}

	attachIaCRelation(
		&report, byID, nameByID, resByCanonRef,
		"ent-src", "aws_iam_role.shared", "USES", nil,
	)
	if len(src.Relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(src.Relations))
	}
	if src.Relations[0].TargetEntityID != "" {
		t.Fatalf("ambiguous ref bound to %q, want empty chip", src.Relations[0].TargetEntityID)
	}
}
