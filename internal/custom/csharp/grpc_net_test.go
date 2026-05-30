package csharp_test

// ---------------------------------------------------------------------------
// gRPC-net — Schema / Codegen / Transport
// ---------------------------------------------------------------------------

import "testing"

func TestGRPCNetProtoContract(t *testing.T) {
	src := `
using ProtoBuf;

[ProtoContract]
public partial class OrderRequest
{
    [ProtoMember(1)]
    public string OrderId { get; set; }

    [ProtoMember(2)]
    public decimal Amount { get; set; }
}
`
	ents := extract(t, "custom_csharp_grpc_net", fi("OrderRequest.cs", "csharp", src))

	foundProc := false
	foundMember := false
	for _, e := range ents {
		if e.Subtype == "procedure_extraction" && e.Kind == "SCOPE.Schema" {
			foundProc = true
		}
		if e.Subtype == "procedure_extraction" && e.Name == "proto_member:OrderId:1" {
			foundMember = true
		}
	}
	if !foundProc {
		t.Error("expected procedure_extraction from [ProtoContract] class")
	}
	if !foundMember {
		t.Error("expected procedure_extraction from [ProtoMember]")
	}
}

func TestGRPCNetProtoFileService(t *testing.T) {
	src := `
syntax = "proto3";

service OrderService {
    rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
    rpc GetOrder(GetOrderRequest) returns (OrderResponse);
}

message CreateOrderRequest {
    string customer_id = 1;
    repeated OrderItem items = 2;
}

message OrderResponse {
    string order_id = 1;
    string status = 2;
}
`
	// Proto files are indexed as csharp in some configurations; test both detection paths
	ents := extract(t, "custom_csharp_grpc_net", fi("order.proto", "csharp", src))

	foundService := false
	foundRPC := false
	foundMessage := false
	for _, e := range ents {
		if e.Subtype == "procedure_extraction" && e.Kind == "SCOPE.Schema" && e.Name == "service:OrderService" {
			foundService = true
		}
		if e.Subtype == "procedure_extraction" && e.Name == "rpc:CreateOrder" {
			foundRPC = true
		}
		if e.Subtype == "schema_extraction" && e.Name == "message:CreateOrderRequest" {
			foundMessage = true
		}
	}
	if !foundService {
		t.Error("expected procedure_extraction for proto service declaration")
	}
	if !foundRPC {
		t.Error("expected procedure_extraction for rpc definition")
	}
	if !foundMessage {
		t.Error("expected schema_extraction for proto message")
	}
}

func TestGRPCNetServiceBase(t *testing.T) {
	src := `
public class OrderServiceImpl : OrderService.OrderServiceBase
{
    public override async Task<CreateOrderResponse> CreateOrder(
        CreateOrderRequest request, ServerCallContext context)
    {
        return new CreateOrderResponse { OrderId = Guid.NewGuid().ToString() };
    }
}
`
	ents := extract(t, "custom_csharp_grpc_net", fi("OrderServiceImpl.cs", "csharp", src))

	foundSchema := false
	for _, e := range ents {
		if e.Subtype == "schema_extraction" && e.Name == "service_impl:OrderServiceImpl" {
			foundSchema = true
			break
		}
	}
	if !foundSchema {
		t.Error("expected schema_extraction from XxxServiceBase subclass")
	}
}

func TestGRPCNetClientCodegen(t *testing.T) {
	src := `
using Grpc.Net.Client;

var channel = GrpcChannel.ForAddress("https://localhost:5001");
var client = new OrderServiceClient(channel);
var response = await client.CreateOrderAsync(request);
`
	ents := extract(t, "custom_csharp_grpc_net", fi("Program.cs", "csharp", src))

	foundChannel := false
	foundClient := false
	for _, e := range ents {
		if e.Subtype == "client_codegen" && e.Kind == "SCOPE.Component" {
			if e.Name == "channel:https://localhost:5001" {
				foundChannel = true
			}
			if e.Name == "client:OrderServiceClient" {
				foundClient = true
			}
		}
	}
	if !foundChannel {
		t.Error("expected client_codegen from GrpcChannel.ForAddress")
	}
	if !foundClient {
		t.Error("expected client_codegen from new XxxClient(channel)")
	}
}

func TestGRPCNetAddGrpcClient(t *testing.T) {
	src := `
builder.Services.AddGrpcClient<OrderServiceClient>(o =>
{
    o.Address = new Uri("https://localhost:5001");
});
`
	ents := extract(t, "custom_csharp_grpc_net", fi("Program.cs", "csharp", src))

	foundClient := false
	for _, e := range ents {
		if e.Subtype == "client_codegen" && e.Kind == "SCOPE.Component" {
			foundClient = true
			break
		}
	}
	if !foundClient {
		t.Error("expected client_codegen from AddGrpcClient<T>")
	}
}

func TestGRPCNetTransportBinding(t *testing.T) {
	src := `
var builder = WebApplication.CreateBuilder(args);
builder.Services.AddGrpc();
var app = builder.Build();
app.MapGrpcService<OrderServiceImpl>();
app.Run();
`
	ents := extract(t, "custom_csharp_grpc_net", fi("Program.cs", "csharp", src))

	foundEndpoint := false
	foundAddGrpc := false
	for _, e := range ents {
		if e.Subtype == "transport_binding" {
			if e.Name == "grpc_endpoint:OrderServiceImpl" {
				foundEndpoint = true
			}
			if e.Kind == "SCOPE.Pattern" {
				foundAddGrpc = true
			}
		}
	}
	if !foundEndpoint {
		t.Error("expected transport_binding from MapGrpcService<T>")
	}
	if !foundAddGrpc {
		t.Error("expected transport_binding from AddGrpc()")
	}
}

func TestGRPCNetDataContract(t *testing.T) {
	src := `
[DataContract]
public class ProductDto
{
    [DataMember(Order = 1)]
    public string Name { get; set; }
}
`
	ents := extract(t, "custom_csharp_grpc_net", fi("ProductDto.cs", "csharp", src))

	foundSchema := false
	for _, e := range ents {
		if e.Subtype == "schema_extraction" && e.Name == "datacontract:ProductDto" {
			foundSchema = true
			break
		}
	}
	if !foundSchema {
		t.Error("expected schema_extraction from [DataContract] class")
	}
}

func TestGRPCNetNoMatch(t *testing.T) {
	src := `namespace MyApp { class Helper { } }`
	ents := extract(t, "custom_csharp_grpc_net", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
