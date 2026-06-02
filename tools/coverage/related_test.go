package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testRegistryWithDatastores builds a small registry exercising the
// cross-link logic: a MongoDB infra record plus mongoose/mongoengine/go
// driver records, a Neo4j infra record plus neomodel/neogma/grafeo
// records, and a ClickHouse infra record with NO related driver/ORM
// records (negative case).
func testRegistryWithDatastores() *Registry {
	return &Registry{
		SchemaVersion: SchemaVersion,
		Records: []Record{
			{ID: "db.mongodb", Category: "databases", Language: "multi", Label: "MongoDB (collections)",
				Capabilities: map[string]Capability{
					"resource_extraction":    {Status: StatusPartial},
					"dependency_attribution": {Status: StatusPartial},
				}},
			{ID: "lang.jsts.orm.mongoose", Category: "orm", Language: "jsts", Label: "Mongoose",
				Groups: map[string]map[string]Capability{
					"Models":        {"a": {Status: StatusFull}, "b": {Status: StatusFull}, "c": {Status: StatusFull}},
					"Relationships": {"d": {Status: StatusPartial}, "e": {Status: StatusPartial}},
				}},
			{ID: "lang.python.orm.mongoengine", Category: "orm", Language: "python", Label: "MongoEngine",
				Capabilities: map[string]Capability{
					"a": {Status: StatusPartial}, "b": {Status: StatusPartial},
				}},
			{ID: "lang.go.driver.mongodb", Category: "orm", Language: "go", Label: "mongo-go-driver",
				Capabilities: map[string]Capability{
					"a": {Status: StatusFull}, "b": {Status: StatusMissing},
				}},
			{ID: "db.neo4j", Category: "databases", Language: "multi", Label: "Neo4j",
				Capabilities: map[string]Capability{
					"resource_extraction":    {Status: StatusMissing},
					"dependency_attribution": {Status: StatusMissing},
				}},
			{ID: "lang.python.orm.neomodel", Category: "orm", Language: "python", Label: "neomodel",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}}},
			{ID: "lang.jsts.orm.neogma", Category: "orm", Language: "jsts", Label: "neogma",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
			{ID: "lang.java.orm.grafeo", Category: "orm", Language: "java", Label: "grafeo",
				Capabilities: map[string]Capability{"a": {Status: StatusMissing}}},
			// ClickHouse infra record with no related driver/ORM records.
			{ID: "db.clickhouse", Category: "databases", Language: "multi", Label: "ClickHouse",
				Capabilities: map[string]Capability{
					"resource_extraction": {Status: StatusMissing},
				}},
			// A general-SQL ORM that must NOT be linked to any datastore.
			{ID: "lang.go.orm.gorm", Category: "orm", Language: "go", Label: "GORM",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}}},
		},
	}
}

func TestStatusDigest(t *testing.T) {
	rec := Record{Groups: map[string]map[string]Capability{
		"G": {
			"a": {Status: StatusFull}, "b": {Status: StatusFull}, "c": {Status: StatusFull},
			"d": {Status: StatusPartial}, "e": {Status: StatusPartial},
			"f": {Status: StatusMissing},
			"g": {Status: StatusNotApplicable},
		},
	}}
	got := statusDigest(rec)
	want := "3 full, 2 partial, 1 missing, 1 n/a"
	if got != want {
		t.Fatalf("statusDigest = %q, want %q", got, want)
	}
	if got := statusDigest(Record{}); got != "no cells" {
		t.Fatalf("empty statusDigest = %q, want %q", got, "no cells")
	}
}

func TestRelatedDriverORMRecords(t *testing.T) {
	reg := testRegistryWithDatastores()
	mongo := reg.Records[0] // db.mongodb
	related := relatedDriverORMRecords(mongo, reg.Records)
	ids := make([]string, len(related))
	for i, r := range related {
		ids[i] = r.ID
	}
	for _, want := range []string{"lang.jsts.orm.mongoose", "lang.python.orm.mongoengine", "lang.go.driver.mongodb"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
			}
		}
		if !found {
			t.Errorf("db.mongodb related missing %s; got %v", want, ids)
		}
	}
	// gorm (general SQL ORM) must never be linked under a datastore.
	for _, id := range ids {
		if strings.Contains(id, "gorm") {
			t.Errorf("gorm wrongly linked under db.mongodb: %v", ids)
		}
	}
	// Digest count assertion for mongoose (3 full + 2 partial).
	for _, r := range related {
		if r.ID == "lang.jsts.orm.mongoose" {
			if r.Digest != "3 full, 2 partial" {
				t.Errorf("mongoose digest = %q, want %q", r.Digest, "3 full, 2 partial")
			}
		}
	}

	// Neo4j: neomodel/neogma/grafeo all associate.
	neo := reg.Records[4] // db.neo4j
	neoRelated := relatedDriverORMRecords(neo, reg.Records)
	neoIDs := map[string]bool{}
	for _, r := range neoRelated {
		neoIDs[r.ID] = true
	}
	for _, want := range []string{"lang.python.orm.neomodel", "lang.jsts.orm.neogma", "lang.java.orm.grafeo"} {
		if !neoIDs[want] {
			t.Errorf("db.neo4j related missing %s; got %v", want, neoIDs)
		}
	}

	// ClickHouse: no related records → nil section.
	ch := reg.Records[8]
	if got := relatedDriverORMRecords(ch, reg.Records); got != nil {
		t.Errorf("db.clickhouse should have no related records, got %v", got)
	}
}

func TestInfraRecordFor(t *testing.T) {
	reg := testRegistryWithDatastores()
	mongoose := reg.Records[1]
	infra := infraRecordFor(mongoose, reg.Records)
	if infra == nil || infra.ID != "db.mongodb" {
		t.Fatalf("mongoose infra = %v, want db.mongodb", infra)
	}
	// gorm (general SQL) targets no recognised datastore → nil.
	gorm := reg.Records[9]
	if infra := infraRecordFor(gorm, reg.Records); infra != nil {
		t.Errorf("gorm should have no infra record, got %v", infra)
	}
	// An infra record itself is not a driver/ORM → nil.
	if infra := infraRecordFor(reg.Records[0], reg.Records); infra != nil {
		t.Errorf("db.mongodb should not back-link to an infra record, got %v", infra)
	}
}

// TestGenRelatedRecordsSection is the end-to-end gen-level assertion:
// db.mongodb's page lists the mongoose/mongoengine/go-driver records with
// links and a digest; mongoose backlinks to db.mongodb; db.clickhouse has
// no Code-level section (negative).
func TestGenRelatedRecordsSection(t *testing.T) {
	reg := testRegistryWithDatastores()
	root := t.TempDir()
	if err := generate(reg, root); err != nil {
		t.Fatalf("generate: %v", err)
	}
	read := func(id string) string {
		data, err := os.ReadFile(filepath.Join(root, "docs", "coverage", "detail", id+".md"))
		if err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		return string(data)
	}

	mongo := read("db.mongodb")
	if !strings.Contains(mongo, "## Related extraction records") {
		t.Errorf("db.mongodb missing Related extraction records section:\n%s", mongo)
	}
	for _, want := range []string{
		"[`lang.jsts.orm.mongoose`](./lang.jsts.orm.mongoose.md)",
		"[`lang.python.orm.mongoengine`](./lang.python.orm.mongoengine.md)",
		"[`lang.go.driver.mongodb`](./lang.go.driver.mongodb.md)",
		"3 full, 2 partial", // mongoose digest
	} {
		if !strings.Contains(mongo, want) {
			t.Errorf("db.mongodb missing %q:\n%s", want, mongo)
		}
	}
	if strings.Contains(mongo, "gorm") {
		t.Errorf("db.mongodb wrongly lists gorm:\n%s", mongo)
	}

	// neo4j lists neomodel/neogma/grafeo.
	neo := read("db.neo4j")
	for _, want := range []string{"neomodel", "neogma", "grafeo"} {
		if !strings.Contains(neo, want) {
			t.Errorf("db.neo4j missing %q:\n%s", want, neo)
		}
	}

	// mongoose backlinks to db.mongodb.
	mg := read("lang.jsts.orm.mongoose")
	if !strings.Contains(mg, "## Related extraction records") {
		t.Errorf("mongoose missing back-link section:\n%s", mg)
	}
	if !strings.Contains(mg, "[`db.mongodb`](./db.mongodb.md)") {
		t.Errorf("mongoose missing db.mongodb back-link:\n%s", mg)
	}

	// clickhouse: no related section (negative).
	ch := read("db.clickhouse")
	if strings.Contains(ch, "## Related extraction records") {
		t.Errorf("db.clickhouse should have no Related extraction records section:\n%s", ch)
	}

	// gorm: no back-link (negative).
	gorm := read("lang.go.orm.gorm")
	if strings.Contains(gorm, "## Related extraction records") {
		t.Errorf("gorm should have no back-link:\n%s", gorm)
	}
}

// testRegistryGeneralizedHubs builds a registry exercising the
// generalized hub→spoke cross-link beyond databases: a gRPC protocol hub
// with per-language http_framework spokes, a Kafka broker hub with
// per-language client spokes, a Prometheus observability hub with a
// Micrometer facade spoke, and a SOAP hub with NO spokes (negative).
func testRegistryGeneralizedHubs() *Registry {
	return &Registry{
		SchemaVersion: SchemaVersion,
		Records: []Record{
			// protocol.grpc hub + per-language gRPC framework spokes.
			{ID: "protocol.grpc", Category: "protocol", Language: "multi", Label: "gRPC",
				Capabilities: map[string]Capability{"resource_extraction": {Status: StatusPartial}}},
			{ID: "lang.rust.framework.tonic", Category: "http_framework", Language: "rust", Label: "Tonic",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}, "b": {Status: StatusFull}, "c": {Status: StatusPartial}}},
			{ID: "lang.csharp.framework.grpc-net", Category: "http_framework", Language: "csharp", Label: "grpc-dotnet",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
			// A non-gRPC http_framework that must NOT be linked under grpc.
			{ID: "lang.go.framework.gin", Category: "http_framework", Language: "go", Label: "Gin",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}}},

			// protocol.soap hub with no spokes (negative).
			{ID: "protocol.soap", Category: "protocol", Language: "multi", Label: "SOAP / WSDL",
				Capabilities: map[string]Capability{"resource_extraction": {Status: StatusMissing}}},

			// msg.broker.kafka hub + per-language Kafka client spokes.
			{ID: "msg.broker.kafka", Category: "message_broker", Language: "multi", Label: "Apache Kafka",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
			{ID: "lang.rust.framework.rdkafka", Category: "message_broker", Language: "rust", Label: "rdkafka (Kafka)",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}, "b": {Status: StatusFull}}},
			{ID: "lang.c-cpp.framework.librdkafka", Category: "message_broker", Language: "c-cpp", Label: "librdkafka",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
			// A non-Kafka broker client that must NOT be linked under kafka.
			{ID: "lang.rust.framework.lapin", Category: "message_broker", Language: "rust", Label: "lapin (AMQP/RabbitMQ)",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}}},
			{ID: "msg.broker.rabbitmq", Category: "message_broker", Language: "multi", Label: "RabbitMQ",
				Capabilities: map[string]Capability{"a": {Status: StatusMissing}}},

			// observability: Prometheus hub + Micrometer facade spoke.
			{ID: "infra.observability.prometheus", Category: "observability", Language: "multi", Label: "Prometheus",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
			{ID: "infra.observability.micrometer", Category: "observability", Language: "multi", Label: "Micrometer",
				Capabilities: map[string]Capability{"a": {Status: StatusFull}, "b": {Status: StatusPartial}}},
			// A vendor with no spokes (negative).
			{ID: "infra.observability.datadog", Category: "observability", Language: "multi", Label: "Datadog",
				Capabilities: map[string]Capability{"a": {Status: StatusPartial}}},
		},
	}
}

// TestGeneralizedHubSpokeResolver is the value-asserting end-to-end test
// for the generalized cross-link: protocol.grpc lists tonic/grpc-net with
// a digest and tonic backlinks; msg.broker.kafka lists rdkafka/librdkafka;
// Prometheus lists Micrometer; SOAP and Datadog (no spokes) render no
// section. Negative: gin/lapin are not mis-attributed.
func TestGeneralizedHubSpokeResolver(t *testing.T) {
	reg := testRegistryGeneralizedHubs()
	root := t.TempDir()
	if err := generate(reg, root); err != nil {
		t.Fatalf("generate: %v", err)
	}
	read := func(id string) string {
		data, err := os.ReadFile(filepath.Join(root, "docs", "coverage", "detail", id+".md"))
		if err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		return string(data)
	}

	// protocol.grpc lists its per-language gRPC frameworks with a digest.
	grpc := read("protocol.grpc")
	if !strings.Contains(grpc, "## Related extraction records") {
		t.Errorf("protocol.grpc missing Related extraction records section:\n%s", grpc)
	}
	for _, want := range []string{
		"[`lang.rust.framework.tonic`](./lang.rust.framework.tonic.md)",
		"[`lang.csharp.framework.grpc-net`](./lang.csharp.framework.grpc-net.md)",
		"2 full, 1 partial", // tonic digest
	} {
		if !strings.Contains(grpc, want) {
			t.Errorf("protocol.grpc missing %q:\n%s", want, grpc)
		}
	}
	if strings.Contains(grpc, "framework.gin") {
		t.Errorf("protocol.grpc wrongly lists gin (non-gRPC framework):\n%s", grpc)
	}
	// tonic backlinks to protocol.grpc.
	tonic := read("lang.rust.framework.tonic")
	if !strings.Contains(tonic, "[`protocol.grpc`](./protocol.grpc.md)") {
		t.Errorf("tonic missing protocol.grpc back-link:\n%s", tonic)
	}

	// protocol.soap: no spokes → no section (negative).
	soap := read("protocol.soap")
	if strings.Contains(soap, "## Related extraction records") {
		t.Errorf("protocol.soap should have no Related extraction records section:\n%s", soap)
	}

	// msg.broker.kafka lists rdkafka/librdkafka; not lapin.
	kafka := read("msg.broker.kafka")
	for _, want := range []string{
		"[`lang.rust.framework.rdkafka`](./lang.rust.framework.rdkafka.md)",
		"[`lang.c-cpp.framework.librdkafka`](./lang.c-cpp.framework.librdkafka.md)",
		"2 full", // rdkafka digest
	} {
		if !strings.Contains(kafka, want) {
			t.Errorf("msg.broker.kafka missing %q:\n%s", want, kafka)
		}
	}
	if strings.Contains(kafka, "lapin") {
		t.Errorf("msg.broker.kafka wrongly lists lapin (RabbitMQ client):\n%s", kafka)
	}
	// rdkafka backlinks to the kafka hub.
	rdkafka := read("lang.rust.framework.rdkafka")
	if !strings.Contains(rdkafka, "[`msg.broker.kafka`](./msg.broker.kafka.md)") {
		t.Errorf("rdkafka missing msg.broker.kafka back-link:\n%s", rdkafka)
	}
	// lapin backlinks to rabbitmq (not kafka).
	lapin := read("lang.rust.framework.lapin")
	if !strings.Contains(lapin, "[`msg.broker.rabbitmq`](./msg.broker.rabbitmq.md)") {
		t.Errorf("lapin missing msg.broker.rabbitmq back-link:\n%s", lapin)
	}

	// observability: Prometheus lists Micrometer.
	prom := read("infra.observability.prometheus")
	if !strings.Contains(prom, "[`infra.observability.micrometer`](./infra.observability.micrometer.md)") {
		t.Errorf("prometheus missing micrometer spoke:\n%s", prom)
	}
	if !strings.Contains(prom, "1 full, 1 partial") {
		t.Errorf("prometheus missing micrometer digest:\n%s", prom)
	}
	// datadog: no spokes → no section (negative).
	dd := read("infra.observability.datadog")
	if strings.Contains(dd, "## Related extraction records") {
		t.Errorf("datadog should have no Related extraction records section:\n%s", dd)
	}
}

// TestGeneralizedHubIdempotent guards that generating twice yields byte-
// identical hub pages (deterministic/idempotent).
func TestGeneralizedHubIdempotent(t *testing.T) {
	reg := testRegistryGeneralizedHubs()
	r1, r2 := t.TempDir(), t.TempDir()
	if err := generate(reg, r1); err != nil {
		t.Fatalf("generate r1: %v", err)
	}
	if err := generate(reg, r2); err != nil {
		t.Fatalf("generate r2: %v", err)
	}
	for _, id := range []string{"protocol.grpc", "msg.broker.kafka", "infra.observability.prometheus", "lang.rust.framework.tonic"} {
		a, err := os.ReadFile(filepath.Join(r1, "docs", "coverage", "detail", id+".md"))
		if err != nil {
			t.Fatalf("read r1 %s: %v", id, err)
		}
		b, err := os.ReadFile(filepath.Join(r2, "docs", "coverage", "detail", id+".md"))
		if err != nil {
			t.Fatalf("read r2 %s: %v", id, err)
		}
		if string(a) != string(b) {
			t.Errorf("non-idempotent output for %s", id)
		}
	}
}

// TestCrossLinkNoCollision guards that no spoke record in the LIVE
// registry associates with more than one hub — a collision would mean a
// match substring is too broad and mis-attributes coverage.
func TestCrossLinkNoCollision(t *testing.T) {
	reg, err := loadRegistry(filepath.Join("..", "..", "docs", "coverage", "registry.json"))
	if err != nil {
		t.Skipf("live registry unavailable: %v", err)
	}
	for _, r := range reg.Records {
		var hits []string
		for _, g := range crossLinkGroups {
			if spokeMatches(g, r) {
				hits = append(hits, g.HubID)
			}
		}
		if len(hits) > 1 {
			t.Errorf("record %s matches multiple hubs %v (match too broad)", r.ID, hits)
		}
	}
}

// TestDatastoreAliasesNoCollision guards that no driver/ORM record in the
// LIVE registry associates with more than one datastore — a collision
// would mean an alias substring is too broad and mis-attributes coverage.
func TestDatastoreAliasesNoCollision(t *testing.T) {
	reg, err := loadRegistry(filepath.Join("..", "..", "docs", "coverage", "registry.json"))
	if err != nil {
		t.Skipf("live registry unavailable: %v", err)
	}
	for _, r := range reg.Records {
		if recordKind(r.ID) == "" {
			continue
		}
		var hits []string
		for db := range datastoreAliases {
			if matchesDatastore(db, r.ID) {
				hits = append(hits, db)
			}
		}
		if len(hits) > 1 {
			t.Errorf("record %s matches multiple datastores %v (alias too broad)", r.ID, hits)
		}
	}
}
