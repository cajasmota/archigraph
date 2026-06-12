// Package csharp — WCF (Windows Communication Foundation) extractor for C#.
//
// Covers the service-contract surface for lang.csharp.framework.wcf (issue
// #4968). WCF models RPC the way gRPC-net does: a [ServiceContract] interface
// declares the service, each [OperationContract] method is an invokable
// procedure, [DataContract]/[DataMember] types describe the payload schema, and
// ServiceHost / AddServiceModelServices registrations bind the service to a
// transport. We mirror the grpc_net.go shape so the two RPC frameworks read the
// same in the graph.
//
//	Schema/procedure_extraction:
//	  [OperationContract] methods inside a [ServiceContract] interface emitted
//	  as SCOPE.Schema/procedure_extraction (one per operation). [ServiceContract]
//	  interfaces/classes emitted as the owning service procedure surface.
//
//	Schema/schema_extraction:
//	  [DataContract] C# classes and their [DataMember] properties emitted as
//	  SCOPE.Schema/schema_extraction.
//
//	Transport/transport_binding:
//	  new ServiceHost(typeof(X)) host registration and
//	  builder.Services.AddServiceModelServices()/AddServiceModelWebServices()
//	  (CoreWCF) emitted as SCOPE.Pattern/transport_binding.
//
//	Codegen/client_codegen (#5004):
//	  WCF client proxies emitted as SCOPE.Component/client_codegen, mirroring the
//	  grpc_net.go client surface. Three idioms are recognised:
//	    - new ChannelFactory<IContract>(...) — the factory carries a USES edge to
//	      the consumed service contract (contract:<IContract>).
//	    - class XxxClient : ClientBase<IContract> — a generated proxy class, with
//	      a USES edge to its contract type argument.
//	    - new XxxClient(...) — instantiation of a generated ClientBase proxy.
//	  These are the WCF analogue of an outbound RPC call into a [ServiceContract].
//
// Registration key: "custom_csharp_wcf"
// Issues #4968, #5004.
package csharp

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_csharp_wcf", &wcfExtractor{})
}

type wcfExtractor struct{}

func (e *wcfExtractor) Language() string { return "custom_csharp_wcf" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// [ServiceContract] on an interface or class — declares a WCF service.
	// Captures the type name (the leading I on interfaces is part of the name).
	reWCFServiceContract = regexp.MustCompile(
		`\[ServiceContract\b[^\]]*\]\s*(?:public\s+|internal\s+)?(?:partial\s+)?(?:interface|class)\s+(\w+)`,
	)

	// [OperationContract] on a method — one invokable RPC operation. Captures
	// the method name. The return type may be a generic (Task<T>) so we skip it.
	reWCFOperationContract = regexp.MustCompile(
		`\[OperationContract\b[^\]]*\]\s*(?:public\s+|internal\s+)?(?:async\s+)?[\w.<>\[\]]+(?:\s*<[^>]+>)?\s+(\w+)\s*\(`,
	)

	// [DataContract] C# class — WCF payload schema type.
	reWCFDataContract = regexp.MustCompile(
		`\[DataContract\b[^\]]*\]\s*(?:public\s+|internal\s+)?(?:partial\s+)?class\s+(\w+)`,
	)

	// [DataMember] property — a serialized member of a data contract.
	reWCFDataMember = regexp.MustCompile(
		`\[DataMember\b[^\]]*\]\s*(?:public\s+)?[\w.<>\[\]]+(?:\s*<[^>]+>)?\s+(\w+)\s*(?:\{|;|=)`,
	)

	// new ServiceHost(typeof(MyService)) — self-hosted WCF endpoint binding.
	reWCFServiceHost = regexp.MustCompile(
		`new\s+ServiceHost\s*\(\s*typeof\s*\(\s*(\w+)\s*\)`,
	)

	// CoreWCF registration: AddServiceModelServices / AddServiceModelWebServices.
	reWCFAddServiceModel = regexp.MustCompile(
		`\.AddServiceModel(?:Web)?Services\s*\(`,
	)

	// CoreWCF endpoint wiring: builder.AddServiceEndpoint<TService, TContract>()
	// or serviceBuilder.AddServiceEndpoint(...). Captures the contract type when
	// expressed generically.
	reWCFAddServiceEndpoint = regexp.MustCompile(
		`\.AddServiceEndpoint\s*<\s*(\w+)\s*,\s*(\w+)\s*>`,
	)

	// new ChannelFactory<IContract>(...) — the canonical WCF client proxy
	// factory. Captures the consumed service-contract type argument. The leaf
	// type is taken so qualified names (Foo.IBar) normalise to IBar.
	reWCFChannelFactory = regexp.MustCompile(
		`new\s+ChannelFactory\s*<\s*([\w.]+)\s*>`,
	)

	// class XxxClient : ClientBase<IContract> — a generated WCF client proxy
	// class (svcutil/Add Service Reference output). Captures the proxy class name
	// and its contract type argument.
	reWCFClientBase = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*ClientBase\s*<\s*([\w.]+)\s*>`,
	)

	// new XxxClient(...) — instantiation of a generated proxy whose name ends in
	// Client. Cheap heuristic mirroring grpc_net.go's reGRPCClientCtor.
	reWCFClientCtor = regexp.MustCompile(
		`new\s+([\w.]*Client)\s*\(`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *wcfExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.wcf_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "wcf"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "csharp" {
		return nil, nil
	}

	src := string(file.Content)

	// Cheap gate: only WCF files carry these attributes / host types / client
	// proxy idioms.
	if !regexpAny(src, "[ServiceContract", "[OperationContract", "[DataContract",
		"ServiceHost", "AddServiceModel", "ChannelFactory", "ClientBase") {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// -------------------------------------------------------------------------
	// Schema/procedure_extraction — the service + its operations
	// -------------------------------------------------------------------------

	// [ServiceContract] interfaces/classes
	for _, m := range reWCFServiceContract.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("service:"+name, "SCOPE.Schema", "procedure_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_SERVICE_CONTRACT",
			"service_name", name)
		add(ent)
	}

	// [OperationContract] methods
	for _, m := range reWCFOperationContract.FindAllStringSubmatchIndex(src, -1) {
		opName := src[m[2]:m[3]]
		ent := makeEntity("operation:"+opName, "SCOPE.Schema", "procedure_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_OPERATION_CONTRACT",
			"operation_name", opName)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Schema/schema_extraction — data contracts + members
	// -------------------------------------------------------------------------

	// [DataContract] classes
	for _, m := range reWCFDataContract.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("datacontract:"+name, "SCOPE.Schema", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_DATA_CONTRACT",
			"class_name", name)
		add(ent)
	}

	// [DataMember] properties
	for _, m := range reWCFDataMember.FindAllStringSubmatchIndex(src, -1) {
		field := src[m[2]:m[3]]
		ent := makeEntity("datamember:"+field+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Schema", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_DATA_MEMBER",
			"field_name", field)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Transport/transport_binding — host + CoreWCF registration
	// -------------------------------------------------------------------------

	// new ServiceHost(typeof(X)) — self-hosted endpoint
	for _, m := range reWCFServiceHost.FindAllStringSubmatchIndex(src, -1) {
		svc := src[m[2]:m[3]]
		ent := makeEntity("service_host:"+svc, "SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_SERVICE_HOST",
			"service_type", svc)
		add(ent)
	}

	// builder.AddServiceEndpoint<TService, TContract>() — CoreWCF endpoint
	for _, m := range reWCFAddServiceEndpoint.FindAllStringSubmatchIndex(src, -1) {
		svc := src[m[2]:m[3]]
		contract := src[m[4]:m[5]]
		ent := makeEntity("service_endpoint:"+svc+":"+contract, "SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_ADD_SERVICE_ENDPOINT",
			"service_type", svc, "contract_type", contract)
		add(ent)
	}

	// .AddServiceModelServices() / .AddServiceModelWebServices() — CoreWCF wiring
	for _, m := range reWCFAddServiceModel.FindAllStringIndex(src, -1) {
		ent := makeEntity("add_service_model:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_ADD_SERVICE_MODEL")
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Codegen/client_codegen — outbound WCF client proxies (#5004)
	// -------------------------------------------------------------------------

	// new ChannelFactory<IContract>(...) — channel-factory proxy. USES the
	// consumed service contract, the WCF analogue of an outbound RPC call.
	for _, m := range reWCFChannelFactory.FindAllStringSubmatchIndex(src, -1) {
		contract := leafType(src[m[2]:m[3]])
		if contract == "" {
			continue
		}
		line := lineOf(src, m[0])
		ent := makeEntity("channel_factory:"+contract, "SCOPE.Component", "client_codegen", file.Path, "csharp", line)
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_CHANNEL_FACTORY",
			"contract_type", contract, "proxy_kind", "channel_factory")
		ent.Relationships = append(ent.Relationships, types.RelationshipRecord{
			ToID: "contract:" + contract,
			Kind: string(types.RelationshipKindUses),
			Properties: map[string]string{
				"contract_type": contract,
				"framework":     "wcf",
				"proxy_kind":    "channel_factory",
				"line":          itoa(line),
			},
		})
		add(ent)
	}

	// class XxxClient : ClientBase<IContract> — generated proxy class. USES the
	// contract type argument it proxies.
	for _, m := range reWCFClientBase.FindAllStringSubmatchIndex(src, -1) {
		clientName := src[m[2]:m[3]]
		contract := leafType(src[m[4]:m[5]])
		line := lineOf(src, m[0])
		ent := makeEntity("client_base:"+clientName, "SCOPE.Component", "client_codegen", file.Path, "csharp", line)
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_CLIENT_BASE",
			"client_class", clientName, "contract_type", contract, "proxy_kind", "client_base")
		if contract != "" {
			ent.Relationships = append(ent.Relationships, types.RelationshipRecord{
				ToID: "contract:" + contract,
				Kind: string(types.RelationshipKindUses),
				Properties: map[string]string{
					"client_class":  clientName,
					"contract_type": contract,
					"framework":     "wcf",
					"proxy_kind":    "client_base",
					"line":          itoa(line),
				},
			})
		}
		add(ent)
	}

	// new XxxClient(...) — instantiation of a generated proxy. Skip the
	// ChannelFactory ctor (handled above) which also ends in "...Factory", not
	// "...Client", so no overlap there.
	for _, m := range reWCFClientCtor.FindAllStringSubmatchIndex(src, -1) {
		clientName := leafType(src[m[2]:m[3]])
		if clientName == "" || wcfNonProxyClients[clientName] {
			continue
		}
		line := lineOf(src, m[0])
		ent := makeEntity("client:"+clientName, "SCOPE.Component", "client_codegen", file.Path, "csharp", line)
		setProps(&ent, "framework", "wcf", "provenance", "INFERRED_FROM_CLIENT_CTOR",
			"client_class", clientName, "proxy_kind", "client_ctor")
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// wcfNonProxyClients are common .NET client types that end in "Client" but are
// not WCF generated proxies — excluded from the new XxxClient(...) heuristic so
// the client_codegen surface stays WCF-specific.
var wcfNonProxyClients = map[string]bool{
	"HttpClient":        true,
	"WebClient":         true,
	"TcpClient":         true,
	"UdpClient":         true,
	"SmtpClient":        true,
	"GrpcClient":        true,
	"HttpMessageClient": true,
	"RestClient":        true,
	"BlobClient":        true,
	"QueueClient":       true,
	"ServiceBusClient":  true,
}

// regexpAny reports whether src contains any of the literal substrings. Used as
// a cheap pre-filter before running the WCF regex catalog.
func regexpAny(src string, subs ...string) bool {
	for _, s := range subs {
		if strings.Contains(src, s) {
			return true
		}
	}
	return false
}
