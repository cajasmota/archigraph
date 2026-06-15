// Tests for gRPC-Java server-auth detection (#4041, epic #3872).
//
// VALUE-ASSERTING: each positive asserts auth_required=true + the specific
// auth_method / auth_roles on the SPECIFIC gRPC method served by an
// auth-enforcing ServerInterceptor or carrying a Spring/Jakarta-Security
// annotation (not len>0). Negatives prove a logging interceptor, a plain
// @GrpcService method, and a no-interceptor server leave the methods UNSTAMPED.
package engine

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

func javaGRPCMethodEntity(ents []types.EntityRecord, service, method string) *types.EntityRecord {
	want := "grpc:" + service + "/" + method
	for i := range ents {
		if ents[i].Kind == grpcMethodKind && ents[i].Name == want {
			return &ents[i]
		}
	}
	return nil
}

func javaGRPCServiceEntity(ents []types.EntityRecord, service string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == grpcServiceKind && ents[i].Name == service {
			return &ents[i]
		}
	}
	return nil
}

// requireJavaGRPCInterceptorAuth asserts the method + its service carry the
// interceptor auth contract.
func requireJavaGRPCInterceptorAuth(t *testing.T, ents []types.EntityRecord, service, method, wantSymbol, wantConf string) {
	t.Helper()
	m := javaGRPCMethodEntity(ents, service, method)
	if m == nil {
		t.Fatalf("expected GrpcMethod grpc:%s/%s; got %v", service, method, ents)
	}
	if got := m.Properties["auth_required"]; got != "true" {
		t.Errorf("method %s/%s: auth_required = %q, want true", service, method, got)
	}
	if got := m.Properties["auth_method"]; got != grpcJavaInterceptorAuthMethod {
		t.Errorf("method %s/%s: auth_method = %q, want %q", service, method, got, grpcJavaInterceptorAuthMethod)
	}
	if got := m.Properties["auth_middleware"]; got != wantSymbol {
		t.Errorf("method %s/%s: auth_middleware = %q, want %q", service, method, got, wantSymbol)
	}
	if got := m.Properties["auth_confidence"]; got != wantConf {
		t.Errorf("method %s/%s: auth_confidence = %q, want %q", service, method, got, wantConf)
	}
	policy := DecodeAuthPolicy(m.Properties["auth_policy"])
	if !policy.Required || policy.Method != "middleware" {
		t.Errorf("method %s/%s: auth_policy = %+v, want required middleware", service, method, policy)
	}
	if svc := javaGRPCServiceEntity(ents, service); svc != nil {
		if svc.Properties["auth_required"] != "true" {
			t.Errorf("service %s: auth_required = %q, want true", service, svc.Properties["auth_required"])
		}
	}
}

func requireNoJavaGRPCAuth(t *testing.T, ents []types.EntityRecord, service, method string) {
	t.Helper()
	m := javaGRPCMethodEntity(ents, service, method)
	if m == nil {
		t.Fatalf("expected GrpcMethod grpc:%s/%s; got %v", service, method, ents)
	}
	for _, k := range []string{"auth_required", "auth_method", "auth_middleware", "auth_policy", "auth_roles"} {
		if v := m.Properties[k]; v != "" {
			t.Errorf("method %s/%s: expected no auth, but %s = %q", service, method, k, v)
		}
	}
}

// --- Path 1: ServerInterceptor auth ---

// TestGRPCJavaAuth_Interceptor_PerService: an in-file AuthInterceptor that
// reads Metadata and closes with Status.UNAUTHENTICATED, bound to the service
// via ServerInterceptors.intercept(service, new AuthInterceptor()).
func TestGRPCJavaAuth_Interceptor_PerService(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.*;
import io.grpc.stub.StreamObserver;
import io.demo.proto.GreeterGrpc;

class AuthInterceptor implements ServerInterceptor {
    static final Metadata.Key<String> AUTHZ =
        Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
    public <Q, A> ServerCall.Listener<Q> interceptCall(
            ServerCall<Q, A> call, Metadata headers, ServerCallHandler<Q, A> next) {
        if (headers.get(AUTHZ) == null) {
            call.close(Status.UNAUTHENTICATED.withDescription("missing token"), new Metadata());
            return new ServerCall.Listener<Q>() {};
        }
        return next.startCall(call, headers);
    }
}

public class GreeterService extends GreeterGrpc.GreeterImplBase {
    @Override
    public void sayHello(HelloRequest req, StreamObserver<HelloReply> observer) {
        observer.onNext(HelloReply.newBuilder().build());
        observer.onCompleted();
    }
    @Override
    public void sayGoodbye(HelloRequest req, StreamObserver<HelloReply> observer) {
        observer.onCompleted();
    }
}

class Server {
    void start() throws Exception {
        ServerBuilder.forPort(8080)
            .addService(ServerInterceptors.intercept(new GreeterService(), new AuthInterceptor()))
            .build().start();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "GreeterService.java", src)
	requireJavaGRPCInterceptorAuth(t, ents, "Greeter", "sayHello", "AuthInterceptor", "high")
	requireJavaGRPCInterceptorAuth(t, ents, "Greeter", "sayGoodbye", "AuthInterceptor", "high")
}

// TestGRPCJavaAuth_Interceptor_GlobalBuilder: a global ServerBuilder.intercept
// binding of an in-file auth interceptor.
func TestGRPCJavaAuth_Interceptor_GlobalBuilder(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.*;
import io.grpc.stub.StreamObserver;
import io.demo.proto.OrderGrpc;

class JwtAuthInterceptor implements ServerInterceptor {
    public <Q, A> ServerCall.Listener<Q> interceptCall(
            ServerCall<Q, A> call, Metadata headers, ServerCallHandler<Q, A> next) {
        String tok = headers.get(Metadata.Key.of("auth", Metadata.ASCII_STRING_MARSHALLER));
        if (!valid(tok)) {
            call.close(Status.PERMISSION_DENIED.withDescription("denied"), new Metadata());
            return new ServerCall.Listener<Q>() {};
        }
        return next.startCall(call, headers);
    }
}

public class OrderService extends OrderGrpc.OrderImplBase {
    @Override
    public void placeOrder(OrderRequest req, StreamObserver<OrderReply> observer) {
        observer.onCompleted();
    }
}

class Boot {
    void run() throws Exception {
        ServerBuilder.forPort(9090)
            .addService(new OrderService())
            .intercept(new JwtAuthInterceptor())
            .build().start();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "OrderService.java", src)
	requireJavaGRPCInterceptorAuth(t, ents, "Order", "placeOrder", "JwtAuthInterceptor", "high")
}

// TestGRPCJavaAuth_Interceptor_ConventionalCrossFile: a bound interceptor whose
// definition is cross-file but is conventionally named — credited MEDIUM.
func TestGRPCJavaAuth_Interceptor_ConventionalCrossFile(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.*;
import io.grpc.stub.StreamObserver;
import io.demo.proto.PayGrpc;
import io.demo.security.AuthServerInterceptor; // defined elsewhere

public class PayService extends PayGrpc.PayImplBase {
    @Override
    public void charge(ChargeRequest req, StreamObserver<ChargeReply> observer) {
        observer.onCompleted();
    }
}

class Boot {
    void run() throws Exception {
        ServerBuilder.forPort(7070)
            .addService(ServerInterceptors.intercept(new PayService(), new AuthServerInterceptor()))
            .build().start();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "PayService.java", src)
	requireJavaGRPCInterceptorAuth(t, ents, "Pay", "charge", "AuthServerInterceptor", "medium")
}

// --- Path 2: Spring/Jakarta-Security annotations ---

// TestGRPCJavaAuth_PreAuthorize_Method: @GrpcService + per-method
// @PreAuthorize("hasRole('ADMIN')") → auth_roles=ADMIN on that method only.
func TestGRPCJavaAuth_PreAuthorize_Method(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.stub.StreamObserver;
import org.springframework.security.access.prepost.PreAuthorize;
import net.devh.boot.grpc.server.service.GrpcService;
import io.demo.proto.AdminGrpc;

@GrpcService
public class AdminService extends AdminGrpc.AdminImplBase {
    @Override
    @PreAuthorize("hasRole('ADMIN')")
    public void deleteAll(Empty req, StreamObserver<Empty> observer) {
        observer.onCompleted();
    }
    @Override
    public void ping(Empty req, StreamObserver<Empty> observer) {
        observer.onCompleted();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "AdminService.java", src)
	m := javaGRPCMethodEntity(ents, "Admin", "deleteAll")
	if m == nil {
		t.Fatalf("expected GrpcMethod grpc:Admin/deleteAll; got %v", ents)
	}
	if m.Properties["auth_required"] != "true" {
		t.Errorf("deleteAll: auth_required = %q, want true", m.Properties["auth_required"])
	}
	if m.Properties["auth_method"] != "annotation" {
		t.Errorf("deleteAll: auth_method = %q, want annotation", m.Properties["auth_method"])
	}
	if m.Properties["auth_roles"] != "ADMIN" {
		t.Errorf("deleteAll: auth_roles = %q, want ADMIN", m.Properties["auth_roles"])
	}
	// ping has no annotation and there is no service-wide interceptor → public.
	requireNoJavaGRPCAuth(t, ents, "Admin", "ping")
}

// TestGRPCJavaAuth_Secured_Method: @Secured("ROLE_MANAGER") → role MANAGER
// (ROLE_ prefix stripped).
func TestGRPCJavaAuth_Secured_Method(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.stub.StreamObserver;
import org.springframework.security.access.annotation.Secured;
import net.devh.boot.grpc.server.service.GrpcService;
import io.demo.proto.ReportGrpc;

@GrpcService
public class ReportService extends ReportGrpc.ReportImplBase {
    @Override
    @Secured("ROLE_MANAGER")
    public void export(ReportRequest req, StreamObserver<ReportReply> observer) {
        observer.onCompleted();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "ReportService.java", src)
	m := javaGRPCMethodEntity(ents, "Report", "export")
	if m == nil {
		t.Fatalf("expected GrpcMethod grpc:Report/export; got %v", ents)
	}
	if m.Properties["auth_required"] != "true" || m.Properties["auth_roles"] != "MANAGER" {
		t.Errorf("export: auth_required=%q auth_roles=%q, want true / MANAGER",
			m.Properties["auth_required"], m.Properties["auth_roles"])
	}
}

// TestGRPCJavaAuth_Authenticated_Class: class-level @Authenticated flows down to
// every method as required (no specific role).
func TestGRPCJavaAuth_Authenticated_Class(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.stub.StreamObserver;
import io.quarkus.security.Authenticated;
import net.devh.boot.grpc.server.service.GrpcService;
import io.demo.proto.AccountGrpc;

@GrpcService
@Authenticated
public class AccountService extends AccountGrpc.AccountImplBase {
    @Override
    public void balance(AccountRequest req, StreamObserver<AccountReply> observer) {
        observer.onCompleted();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "AccountService.java", src)
	m := javaGRPCMethodEntity(ents, "Account", "balance")
	if m == nil {
		t.Fatalf("expected GrpcMethod grpc:Account/balance; got %v", ents)
	}
	if m.Properties["auth_required"] != "true" || m.Properties["auth_method"] != "annotation" {
		t.Errorf("balance: auth_required=%q auth_method=%q, want true / annotation",
			m.Properties["auth_required"], m.Properties["auth_method"])
	}
	if m.Properties["auth_roles"] != "" {
		t.Errorf("balance: auth_roles = %q, want empty (@Authenticated has no role)", m.Properties["auth_roles"])
	}
}

// --- Negatives ---

// TestGRPCJavaAuth_Negative_LoggingInterceptor: a logging interceptor (no
// metadata-reject) bound to the service does NOT enforce auth.
func TestGRPCJavaAuth_Negative_LoggingInterceptor(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.*;
import io.grpc.stub.StreamObserver;
import io.demo.proto.GreeterGrpc;

class LoggingInterceptor implements ServerInterceptor {
    public <Q, A> ServerCall.Listener<Q> interceptCall(
            ServerCall<Q, A> call, Metadata headers, ServerCallHandler<Q, A> next) {
        System.out.println("call: " + call.getMethodDescriptor().getFullMethodName());
        return next.startCall(call, headers);
    }
}

public class GreeterService extends GreeterGrpc.GreeterImplBase {
    @Override
    public void sayHello(HelloRequest req, StreamObserver<HelloReply> observer) {
        observer.onCompleted();
    }
}

class Server {
    void start() throws Exception {
        ServerBuilder.forPort(8080)
            .addService(ServerInterceptors.intercept(new GreeterService(), new LoggingInterceptor()))
            .build().start();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "GreeterService.java", src)
	requireNoJavaGRPCAuth(t, ents, "Greeter", "sayHello")
}

// TestGRPCJavaAuth_Negative_PlainGrpcService: a plain @GrpcService method with
// no interceptor and no security annotation is public.
func TestGRPCJavaAuth_Negative_PlainGrpcService(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import io.demo.proto.EchoGrpc;

@GrpcService
public class EchoService extends EchoGrpc.EchoImplBase {
    @Override
    public void echo(EchoRequest req, StreamObserver<EchoReply> observer) {
        observer.onNext(EchoReply.newBuilder().build());
        observer.onCompleted();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "EchoService.java", src)
	requireNoJavaGRPCAuth(t, ents, "Echo", "echo")
}

// TestGRPCJavaAuth_Negative_NoInterceptorServer: a server with no interceptor
// binding at all leaves methods unstamped.
func TestGRPCJavaAuth_Negative_NoInterceptorServer(t *testing.T) {
	src := `package io.demo.grpc;
import io.grpc.*;
import io.grpc.stub.StreamObserver;
import io.demo.proto.GreeterGrpc;

public class GreeterService extends GreeterGrpc.GreeterImplBase {
    @Override
    public void sayHello(HelloRequest req, StreamObserver<HelloReply> observer) {
        observer.onCompleted();
    }
}

class Server {
    void start() throws Exception {
        ServerBuilder.forPort(8080).addService(new GreeterService()).build().start();
    }
}
`
	ents, _ := runGRPCDetect(t, "java", "GreeterService.java", src)
	requireNoJavaGRPCAuth(t, ents, "Greeter", "sayHello")
}
