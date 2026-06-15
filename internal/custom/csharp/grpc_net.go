// Package csharp — gRPC-net extractor for C# source files.
//
// Covers the four missing cells for lang.csharp.framework.grpc-net:
//
//	Schema/procedure_extraction:
//	  [ProtoMember], [ProtoContract] attributes on C# classes emitted as
//	  SCOPE.Schema/procedure_extraction. proto-file service/rpc declarations
//	  detected by pattern (proto files are parsed as csharp language=csharp
//	  in some setups; we match both).
//
//	Schema/schema_extraction:
//	  [ProtoContract] / [DataContract] C# classes emitted as
//	  SCOPE.Schema/schema_extraction.
//	  *.proto message/service bodies detected via proto syntax keywords.
//
//	Codegen/client_codegen:
//	  GrpcChannel.ForAddress / ChannelBase subclasses emitted as
//	  SCOPE.Component/client_codegen.
//	  Generated stub usages: XxxClient ctors, GrpcClient<T> patterns.
//
//	Transport/transport_binding:
//	  MapGrpcService<T>() endpoint registrations and
//	  ServerCredentials / SslServerCredentials usage emitted as
//	  SCOPE.Pattern/transport_binding.
//
// Registration key: "custom_csharp_grpc_net"
// Issue #3261.
package csharp

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
	extractor.Register("custom_csharp_grpc_net", &grpcNetExtractor{})
}

type grpcNetExtractor struct{}

func (e *grpcNetExtractor) Language() string { return "custom_csharp_grpc_net" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// Schema/procedure_extraction --------------------------------------------

	// [ProtoContract] attribute on a C# class — marks it as a protobuf message
	reGRPCProtoContract = regexp.MustCompile(
		`\[ProtoContract\b[^\]]*\]\s*(?:public\s+)?(?:partial\s+)?class\s+(\w+)`,
	)

	// [ProtoMember(N)] property annotation — marks a field as a proto member
	reGRPCProtoMember = regexp.MustCompile(
		`\[ProtoMember\s*\(\s*(\d+)(?:[^)]*)\)\s*\]\s*(?:public\s+)?(?:\w+(?:<[^>]+>)?)\s+(\w+)`,
	)

	// Proto service definition in .proto files: service Foo { rpc Bar(Req) returns (Res); }
	reGRPCProtoService = regexp.MustCompile(
		`(?m)^service\s+(\w+)\s*\{`,
	)

	// Proto rpc definition: rpc Bar(Baz) returns (Qux);
	reGRPCProtoRPC = regexp.MustCompile(
		`(?m)^\s*rpc\s+(\w+)\s*\(([^)]*)\)\s*returns\s*\(([^)]*)\)`,
	)

	// Schema/schema_extraction -----------------------------------------------

	// Proto message definition: message Foo { ... }
	reGRPCProtoMessage = regexp.MustCompile(
		`(?m)^message\s+(\w+)\s*\{`,
	)

	// [DataContract] C# class — used by WCF/gRPC data contract serialization
	reGRPCDataContract = regexp.MustCompile(
		`\[DataContract\b[^\]]*\]\s*(?:public\s+)?(?:partial\s+)?class\s+(\w+)`,
	)

	// class MyService : MyService.MyServiceBase — generated gRPC service base
	reGRPCServiceBase = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*\w+\.\w+Base\b`,
	)

	// Codegen/client_codegen -------------------------------------------------

	// GrpcChannel.ForAddress("address") — channel creation
	reGRPCChannelForAddress = regexp.MustCompile(
		`GrpcChannel\.ForAddress\s*\(\s*["']([^"']+)["']`,
	)

	// new XxxClient(channel) — generated stub client usage
	// Handles qualified names like OrderService.OrderServiceClient
	reGRPCClientCtor = regexp.MustCompile(
		`new\s+([\w.]+Client)\s*\(`,
	)

	// GrpcClient<T> pattern — generic gRPC client wrapper
	reGRPCClientGeneric = regexp.MustCompile(
		`GrpcClient\s*<\s*([\w.]+)\s*>`,
	)

	// class XxxClient : ClientBase<XxxClient> — generated client class
	reGRPCClientBase = regexp.MustCompile(
		`(?m)class\s+(\w+Client)\s*:\s*ClientBase\b`,
	)

	// Transport/transport_binding --------------------------------------------

	// app.MapGrpcService<T>() — gRPC endpoint registration in ASP.NET Core
	reGRPCMapService = regexp.MustCompile(
		`\.MapGrpcService\s*<\s*(\w+)\s*>`,
	)

	// ServerCredentials / SslServerCredentials — transport security config
	reGRPCServerCredentials = regexp.MustCompile(
		`(?:ServerCredentials|SslServerCredentials|ChannelCredentials|SslCredentials)\s*\.?\s*(?:Insecure|Create)?`,
	)

	// GrpcServiceOptions / GrpcChannelOptions — transport configuration
	reGRPCOptions = regexp.MustCompile(
		`(?:GrpcServiceOptions|GrpcChannelOptions|GrpcClientFactoryOptions)\b`,
	)

	// services.AddGrpc() — gRPC service registration
	reGRPCAddGrpc = regexp.MustCompile(
		`\.AddGrpc\s*\(`,
	)

	// services.AddGrpcClient<T>() — typed gRPC client registration
	// Handles qualified names like OrderService.OrderServiceClient
	reGRPCAddGrpcClient = regexp.MustCompile(
		`\.AddGrpcClient\s*<\s*([\w.]+)\s*>`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *grpcNetExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.grpc_net_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "grpc-net"),
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
	// Schema/procedure_extraction
	// -------------------------------------------------------------------------

	// [ProtoContract] class declarations
	for _, m := range reGRPCProtoContract.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("proto:"+name, "SCOPE.Schema", "procedure_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_PROTO_CONTRACT",
			"message_name", name)
		add(ent)
	}

	// [ProtoMember] properties
	for _, m := range reGRPCProtoMember.FindAllStringSubmatchIndex(src, -1) {
		fieldNum := src[m[2]:m[3]]
		fieldName := src[m[4]:m[5]]
		ent := makeEntity("proto_member:"+fieldName+":"+fieldNum, "SCOPE.Schema", "procedure_extraction",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_PROTO_MEMBER",
			"field_number", fieldNum, "field_name", fieldName)
		add(ent)
	}

	// Proto service definitions
	for _, m := range reGRPCProtoService.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("service:"+name, "SCOPE.Schema", "procedure_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_PROTO_SERVICE",
			"service_name", name)
		add(ent)
	}

	// Proto rpc definitions
	for _, m := range reGRPCProtoRPC.FindAllStringSubmatchIndex(src, -1) {
		rpcName := src[m[2]:m[3]]
		reqType := src[m[4]:m[5]]
		respType := src[m[6]:m[7]]
		ent := makeEntity("rpc:"+rpcName, "SCOPE.Schema", "procedure_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_PROTO_RPC",
			"rpc_name", rpcName, "request_type", reqType, "response_type", respType)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Schema/schema_extraction
	// -------------------------------------------------------------------------

	// Proto message declarations
	for _, m := range reGRPCProtoMessage.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("message:"+name, "SCOPE.Schema", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_PROTO_MESSAGE",
			"message_name", name)
		add(ent)
	}

	// [DataContract] C# classes
	for _, m := range reGRPCDataContract.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("datacontract:"+name, "SCOPE.Schema", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_DATA_CONTRACT",
			"class_name", name)
		add(ent)
	}

	// class XxxService : XxxService.XxxServiceBase — generated service impl
	for _, m := range reGRPCServiceBase.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("service_impl:"+name, "SCOPE.Schema", "schema_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_SERVICE_BASE",
			"class_name", name)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Codegen/client_codegen
	// -------------------------------------------------------------------------

	// GrpcChannel.ForAddress("...") — channel instantiation
	for _, m := range reGRPCChannelForAddress.FindAllStringSubmatchIndex(src, -1) {
		addr := src[m[2]:m[3]]
		ent := makeEntity("channel:"+addr, "SCOPE.Component", "client_codegen", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_CHANNEL",
			"address", addr)
		add(ent)
	}

	// new XxxClient(channel) — generated stub client usage
	for _, m := range reGRPCClientCtor.FindAllStringSubmatchIndex(src, -1) {
		clientName := src[m[2]:m[3]]
		ent := makeEntity("client:"+clientName, "SCOPE.Component", "client_codegen", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_CLIENT_CTOR",
			"client_class", clientName)
		add(ent)
	}

	// class XxxClient : ClientBase<XxxClient> — generated client class
	for _, m := range reGRPCClientBase.FindAllStringSubmatchIndex(src, -1) {
		clientName := src[m[2]:m[3]]
		ent := makeEntity("client_base:"+clientName, "SCOPE.Component", "client_codegen", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_CLIENT_BASE",
			"client_class", clientName)
		add(ent)
	}

	// services.AddGrpcClient<T>() — typed client registration
	for _, m := range reGRPCAddGrpcClient.FindAllStringSubmatchIndex(src, -1) {
		clientType := src[m[2]:m[3]]
		ent := makeEntity("grpc_client:"+clientType, "SCOPE.Component", "client_codegen", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_ADD_GRPC_CLIENT",
			"client_type", clientType)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Transport/transport_binding
	// -------------------------------------------------------------------------

	// app.MapGrpcService<T>() — endpoint registration
	for _, m := range reGRPCMapService.FindAllStringSubmatchIndex(src, -1) {
		serviceType := src[m[2]:m[3]]
		ent := makeEntity("grpc_endpoint:"+serviceType, "SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_MAP_GRPC_SERVICE",
			"service_type", serviceType)
		add(ent)
	}

	// ServerCredentials / SslServerCredentials — security binding
	for _, m := range reGRPCServerCredentials.FindAllStringIndex(src, -1) {
		ent := makeEntity("transport_security:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_CREDENTIALS")
		add(ent)
	}

	// services.AddGrpc() — gRPC service wiring
	for _, m := range reGRPCAddGrpc.FindAllStringIndex(src, -1) {
		ent := makeEntity("add_grpc:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_ADD_GRPC")
		add(ent)
	}

	// GrpcServiceOptions / GrpcChannelOptions / GrpcClientFactoryOptions usage
	for _, m := range reGRPCOptions.FindAllStringIndex(src, -1) {
		ent := makeEntity("grpc_options:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "transport_binding", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "grpc-net", "provenance", "INFERRED_FROM_GRPC_OPTIONS")
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
