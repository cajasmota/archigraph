package cpp_test

// grpc_auth_test.go — value-asserting fixture tests for gRPC C++ (grpc++)
// interceptor / AuthMetadataProcessor auth detection (#4041, epic #3872).
//
// The c-cpp auth_coverage sniffer is HTTP-route/middleware-keyed and emits 0
// auth entities on a gRPC service idiom. gRPC auth lives in a server
// INTERCEPTOR (grpc::experimental::Interceptor wired via SetInterceptorCreators)
// or an AuthMetadataProcessor (Process returning non-OK = auth). These tests
// assert auth_required + auth_method + auth_middleware on the SPECIFIC RPC
// method endpoint guarded by the enforcer — never len>0 — and prove the
// negatives (logging interceptor / no interceptor → UNSTAMPED).

import "testing"

// fixtureInterceptorAuth is a gRPC C++ server whose registered service is
// guarded by a JWT interceptor that reads client metadata and fails with
// UNAUTHENTICATED, wired via SetInterceptorCreators.
const fixtureInterceptorAuth = `
#include <grpcpp/grpcpp.h>
#include <grpcpp/support/server_interceptor.h>
#include "helloworld.grpc.pb.h"
using grpc::ServerContext;
using grpc::Status;

class JwtAuth : public grpc::experimental::Interceptor {
public:
    void Intercept(grpc::experimental::InterceptorBatchMethods* methods) override {
        if (methods->QueryInterceptionHookPoint(
                grpc::experimental::InterceptionHookPoints::POST_RECV_INITIAL_METADATA)) {
            auto* md = methods->GetRecvInitialMetadata();
            auto it = md->find("authorization");
            if (it == md->end() || !verifyJwt(it->second)) {
                methods->ModifySendStatus(
                    grpc::Status(grpc::StatusCode::UNAUTHENTICATED, "missing or invalid token"));
            }
        }
        methods->Proceed();
    }
};

class GreeterServiceImpl final : public Greeter::Service {
    Status SayHello(ServerContext* context, const HelloRequest* request,
                    HelloReply* reply) override {
        return Status::OK;
    }
    Status SayBye(ServerContext* context, const ByeRequest* request,
                  ByeReply* reply) override {
        return Status::OK;
    }
};

void run() {
    grpc::ServerBuilder builder;
    std::vector<std::unique_ptr<grpc::experimental::ServerInterceptorFactoryInterface>> creators;
    creators.push_back(std::make_unique<JwtAuthFactory>());
    builder.experimental().SetInterceptorCreators(std::move(creators));
    GreeterServiceImpl service;
    builder.RegisterService(&service);
}
`

// TestGrpcInterceptorAuth proves a JWT server interceptor → auth stamped on the
// guarded RPC methods with the interceptor class as auth_middleware.
func TestGrpcInterceptorAuth(t *testing.T) {
	ents := extract(t, "custom_cpp_grpc", fi("greeter.cc", "cpp", fixtureInterceptorAuth))

	for _, methodPath := range []string{"RPC /Greeter/SayHello", "RPC /Greeter/SayBye"} {
		ep := findEndpoint(ents, methodPath)
		if ep == nil {
			t.Fatalf("expected %s endpoint, got %+v", methodPath, ents)
		}
		if got := ep.Props["auth_required"]; got != "true" {
			t.Errorf("%s auth_required = %q, want true", methodPath, got)
		}
		if got := ep.Props["auth_method"]; got != "grpc_interceptor" {
			t.Errorf("%s auth_method = %q, want grpc_interceptor", methodPath, got)
		}
		// auth_middleware is the MCP grafel_auth_coverage signal-1 key — it
		// must carry the concrete enforcer class name, not a placeholder.
		if got := ep.Props["auth_middleware"]; got != "JwtAuth" {
			t.Errorf("%s auth_middleware = %q, want JwtAuth", methodPath, got)
		}
		if got := ep.Props["auth_confidence"]; got != "high" {
			t.Errorf("%s auth_confidence = %q, want high", methodPath, got)
		}
		if got := ep.Props["auth_enforcer_kind"]; got != "interceptor" {
			t.Errorf("%s auth_enforcer_kind = %q, want interceptor", methodPath, got)
		}
	}
}

// fixtureMetadataProcessorAuth guards a service via an AuthMetadataProcessor
// whose Process() returns UNAUTHENTICATED on a bad token, wired via
// SetAuthMetadataProcessor on the server credentials.
const fixtureMetadataProcessorAuth = `
#include <grpcpp/grpcpp.h>
#include <grpcpp/security/auth_metadata_processor.h>
#include "routeguide.grpc.pb.h"
using grpc::Status;
using grpc::ServerContext;

class TokenProcessor : public grpc::AuthMetadataProcessor {
public:
    grpc::Status Process(const InputMetadata& auth_metadata, grpc::AuthContext* context,
                         OutputMetadata* consumed_auth_metadata,
                         OutputMetadata* response_metadata) override {
        auto it = auth_metadata.find("authorization");
        if (it == auth_metadata.end() || !validate(it->second)) {
            return grpc::Status(grpc::StatusCode::UNAUTHENTICATED, "bad token");
        }
        return grpc::Status::OK;
    }
};

class RouteGuideImpl final : public routeguide::RouteGuide::Service {
    Status GetFeature(ServerContext* context, const routeguide::Point* request,
                      routeguide::Feature* response) override {
        return Status::OK;
    }
};

void run() {
    auto creds = grpc::SslServerCredentials(opts);
    creds->SetAuthMetadataProcessor(std::make_shared<TokenProcessor>());
    grpc::ServerBuilder builder;
    RouteGuideImpl service;
    builder.RegisterService(&service);
}
`

// TestGrpcMetadataProcessorAuth proves an AuthMetadataProcessor rejecting on a
// bad token → auth stamped on the guarded RPC method.
func TestGrpcMetadataProcessorAuth(t *testing.T) {
	ents := extract(t, "custom_cpp_grpc", fi("routeguide.cc", "cpp", fixtureMetadataProcessorAuth))

	ep := findEndpoint(ents, "RPC /RouteGuide/GetFeature")
	if ep == nil {
		t.Fatalf("expected RPC /RouteGuide/GetFeature endpoint, got %+v", ents)
	}
	if got := ep.Props["auth_required"]; got != "true" {
		t.Errorf("auth_required = %q, want true", got)
	}
	if got := ep.Props["auth_method"]; got != "grpc_interceptor" {
		t.Errorf("auth_method = %q, want grpc_interceptor", got)
	}
	if got := ep.Props["auth_middleware"]; got != "TokenProcessor" {
		t.Errorf("auth_middleware = %q, want TokenProcessor", got)
	}
	if got := ep.Props["auth_enforcer_kind"]; got != "metadata_processor" {
		t.Errorf("auth_enforcer_kind = %q, want metadata_processor", got)
	}
}

// ---- NEGATIVES -----------------------------------------------------------

// fixtureLoggingInterceptor wires an interceptor that reads metadata but never
// rejects with UNAUTHENTICATED/PERMISSION_DENIED — it is observational, NOT
// auth. The guarded method must stay UNSTAMPED.
const fixtureLoggingInterceptor = `
#include <grpcpp/grpcpp.h>
#include <grpcpp/support/server_interceptor.h>
#include "helloworld.grpc.pb.h"
using grpc::ServerContext;
using grpc::Status;

class LoggingInterceptor : public grpc::experimental::Interceptor {
public:
    void Intercept(grpc::experimental::InterceptorBatchMethods* methods) override {
        if (methods->QueryInterceptionHookPoint(
                grpc::experimental::InterceptionHookPoints::POST_RECV_INITIAL_METADATA)) {
            auto* md = methods->GetRecvInitialMetadata();
            LOG(INFO) << "received " << md->size() << " metadata entries";
        }
        methods->Proceed();
    }
};

class GreeterServiceImpl final : public Greeter::Service {
    Status SayHello(ServerContext* context, const HelloRequest* request,
                    HelloReply* reply) override {
        return Status::OK;
    }
};

void run() {
    grpc::ServerBuilder builder;
    std::vector<std::unique_ptr<grpc::experimental::ServerInterceptorFactoryInterface>> creators;
    creators.push_back(std::make_unique<LoggingInterceptorFactory>());
    builder.experimental().SetInterceptorCreators(std::move(creators));
    GreeterServiceImpl service;
    builder.RegisterService(&service);
}
`

// TestGrpcLoggingInterceptorNoAuth proves a logging interceptor (no auth
// reject) leaves the RPC method UNSTAMPED.
func TestGrpcLoggingInterceptorNoAuth(t *testing.T) {
	ents := extract(t, "custom_cpp_grpc", fi("greeter.cc", "cpp", fixtureLoggingInterceptor))

	ep := findEndpoint(ents, "RPC /Greeter/SayHello")
	if ep == nil {
		t.Fatalf("expected RPC /Greeter/SayHello endpoint, got %+v", ents)
	}
	if got := ep.Props["auth_required"]; got != "" {
		t.Errorf("auth_required = %q on a logging interceptor, want unstamped", got)
	}
	if got := ep.Props["auth_middleware"]; got != "" {
		t.Errorf("auth_middleware = %q on a logging interceptor, want unstamped", got)
	}
}

// fixtureNoInterceptor is a plain gRPC server with no interceptor and no
// metadata processor — no auth, method must stay UNSTAMPED.
const fixtureNoInterceptor = `
#include <grpcpp/grpcpp.h>
#include "helloworld.grpc.pb.h"
using grpc::ServerContext;
using grpc::Status;

class GreeterServiceImpl final : public Greeter::Service {
    Status SayHello(ServerContext* context, const HelloRequest* request,
                    HelloReply* reply) override {
        return Status::OK;
    }
};

void run() {
    grpc::ServerBuilder builder;
    GreeterServiceImpl service;
    builder.RegisterService(&service);
}
`

// TestGrpcNoInterceptorNoAuth proves a server with no interceptor leaves the
// RPC method UNSTAMPED.
func TestGrpcNoInterceptorNoAuth(t *testing.T) {
	ents := extract(t, "custom_cpp_grpc", fi("greeter.cc", "cpp", fixtureNoInterceptor))

	ep := findEndpoint(ents, "RPC /Greeter/SayHello")
	if ep == nil {
		t.Fatalf("expected RPC /Greeter/SayHello endpoint, got %+v", ents)
	}
	if got := ep.Props["auth_required"]; got != "" {
		t.Errorf("auth_required = %q on a no-interceptor server, want unstamped", got)
	}
}

// TestGrpcInterceptorPresentButNotWired proves an auth-enforcing interceptor
// class that is declared but NOT wired via SetInterceptorCreators leaves the
// method UNSTAMPED (honest: wiring is required, not just presence).
func TestGrpcInterceptorPresentButNotWired(t *testing.T) {
	src := `
#include <grpcpp/grpcpp.h>
#include <grpcpp/support/server_interceptor.h>
#include "helloworld.grpc.pb.h"
using grpc::ServerContext;
using grpc::Status;

class JwtAuth : public grpc::experimental::Interceptor {
public:
    void Intercept(grpc::experimental::InterceptorBatchMethods* methods) override {
        auto* md = methods->GetRecvInitialMetadata();
        if (!verifyJwt(md)) {
            methods->ModifySendStatus(
                grpc::Status(grpc::StatusCode::UNAUTHENTICATED, "no token"));
        }
        methods->Proceed();
    }
};

class GreeterServiceImpl final : public Greeter::Service {
    Status SayHello(ServerContext* context, const HelloRequest* request,
                    HelloReply* reply) override {
        return Status::OK;
    }
};

void run() {
    grpc::ServerBuilder builder;
    GreeterServiceImpl service;
    builder.RegisterService(&service);
}
`
	ents := extract(t, "custom_cpp_grpc", fi("greeter.cc", "cpp", src))
	ep := findEndpoint(ents, "RPC /Greeter/SayHello")
	if ep == nil {
		t.Fatalf("expected RPC /Greeter/SayHello endpoint, got %+v", ents)
	}
	if got := ep.Props["auth_required"]; got != "" {
		t.Errorf("auth_required = %q on an unwired interceptor, want unstamped", got)
	}
}
