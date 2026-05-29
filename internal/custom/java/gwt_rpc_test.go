package java

import (
	"testing"
)

// ============================================================================
// Issue #3190: GWT RPC / RequestFactory data_fetching extractor
//
// Registry target: lang.java.framework.gwt Data Flow/data_fetching → partial.
// Cite: internal/custom/java/gwt_rpc.go
// ============================================================================

func gwtHasProvenance(r PatternResult, prov string) bool {
	for _, e := range r.Entities {
		if e.Provenance == prov {
			return true
		}
	}
	return false
}

func gwtEntityByProvenance(r PatternResult, prov string) (SecondaryEntity, bool) {
	for _, e := range r.Entities {
		if e.Provenance == prov {
			return e, true
		}
	}
	return SecondaryEntity{}, false
}

// TestGWTRPC_RemoteServiceRelativePath_Issue3190 proves an RPC service
// interface annotated with @RemoteServiceRelativePath is recorded as a
// data_fetching service entity carrying its relative binding path.
func TestGWTRPC_RemoteServiceRelativePath_Issue3190(t *testing.T) {
	source := `
package com.example.client;

import com.google.gwt.user.client.rpc.RemoteService;
import com.google.gwt.user.client.rpc.RemoteServiceRelativePath;

@RemoteServiceRelativePath("greet")
public interface GreetingService extends RemoteService {
    String greetServer(String name);
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "GreetingService.java",
	})

	e, ok := gwtEntityByProvenance(r, "INFERRED_FROM_GWT_RPC_SERVICE")
	if !ok {
		t.Fatalf("[#3190 data_fetching] expected SCOPE.Service from @RemoteServiceRelativePath, got none")
	}
	if e.Name != "GreetingService" {
		t.Errorf("[#3190] expected service name GreetingService, got %q", e.Name)
	}
	if e.Properties["relative_path"] != "greet" {
		t.Errorf("[#3190] expected relative_path=greet, got %v", e.Properties["relative_path"])
	}
}

// TestGWTRPC_GWTCreateProxy_Issue3190 proves GWT.create(Service.class) is
// recorded as a data_fetch proxy-creation operation, linked to the service.
func TestGWTRPC_GWTCreateProxy_Issue3190(t *testing.T) {
	source := `
package com.example.client;

import com.google.gwt.core.client.GWT;

public class GreetingPresenter {
    private final GreetingServiceAsync svc = GWT.create(GreetingService.class);

    public void load() {
        svc.greetServer("world", null);
    }
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "GreetingPresenter.java",
	})

	e, ok := gwtEntityByProvenance(r, "INFERRED_FROM_GWT_CREATE_PROXY")
	if !ok {
		t.Fatalf("[#3190 data_fetching] expected proxy-create operation from GWT.create, got none")
	}
	if e.Properties["service_type"] != "GreetingService" {
		t.Errorf("[#3190] expected service_type=GreetingService, got %v", e.Properties["service_type"])
	}
	foundRel := false
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "FETCHES_FROM" {
			foundRel = true
		}
	}
	if !foundRel {
		t.Errorf("[#3190] expected FETCHES_FROM relationship from proxy-create to service")
	}
}

// TestGWTRPC_AsyncCallback_Issue3190 proves an AsyncCallback<T> completion
// site is recorded as a data_fetch operation carrying its payload type.
func TestGWTRPC_AsyncCallback_Issue3190(t *testing.T) {
	source := `
package com.example.client;

import com.google.gwt.user.client.rpc.AsyncCallback;

public class GreetingPresenter {
    public void load(GreetingServiceAsync svc) {
        svc.greetServer("world", new AsyncCallback<String>() {
            public void onSuccess(String result) {}
            public void onFailure(Throwable caught) {}
        });
    }
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "GreetingPresenter.java",
	})

	e, ok := gwtEntityByProvenance(r, "INFERRED_FROM_GWT_ASYNC_CALLBACK")
	if !ok {
		t.Fatalf("[#3190 data_fetching] expected AsyncCallback data_fetch op, got none")
	}
	if e.Properties["payload_type"] != "String" {
		t.Errorf("[#3190] expected payload_type=String, got %v", e.Properties["payload_type"])
	}
}

// TestGWTRPC_RequestFactory_Issue3190 proves a RequestFactory root interface
// and its RequestContext are recorded as data_fetching services.
func TestGWTRPC_RequestFactory_Issue3190(t *testing.T) {
	source := `
package com.example.client;

import com.google.web.bindery.requestfactory.shared.RequestFactory;
import com.google.web.bindery.requestfactory.shared.RequestContext;
import com.google.web.bindery.requestfactory.shared.Request;

public interface AppRequestFactory extends RequestFactory {
    EmployeeRequest employeeRequest();
}

interface EmployeeRequest extends RequestContext {
    Request<EmployeeProxy> findEmployee(Long id);
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "AppRequestFactory.java",
	})

	if !gwtHasProvenance(r, "INFERRED_FROM_GWT_REQUEST_FACTORY") {
		t.Errorf("[#3190 data_fetching] expected RequestFactory service entity, got none")
	}
	if !gwtHasProvenance(r, "INFERRED_FROM_GWT_REQUEST_CONTEXT") {
		t.Errorf("[#3190 data_fetching] expected RequestContext service entity, got none")
	}
}

// TestGWTRPC_ProxyFor_Issue3190 proves an @ProxyFor EntityProxy bean is
// recorded as a data_fetching DataModel carrying its domain type.
func TestGWTRPC_ProxyFor_Issue3190(t *testing.T) {
	source := `
package com.example.shared;

import com.google.web.bindery.requestfactory.shared.EntityProxy;
import com.google.web.bindery.requestfactory.shared.ProxyFor;

@ProxyFor(Employee.class)
public interface EmployeeProxy extends EntityProxy {
    String getName();
    void setName(String name);
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "EmployeeProxy.java",
	})

	e, ok := gwtEntityByProvenance(r, "INFERRED_FROM_GWT_PROXY")
	if !ok {
		t.Fatalf("[#3190 data_fetching] expected EntityProxy DataModel, got none")
	}
	if e.Name != "EmployeeProxy" {
		t.Errorf("[#3190] expected proxy name EmployeeProxy, got %q", e.Name)
	}
	if e.Properties["domain_type"] != "Employee" {
		t.Errorf("[#3190] expected domain_type=Employee, got %v", e.Properties["domain_type"])
	}
}

// TestGWTRPC_RequestFire_Issue3190 proves request.fire(Receiver) is recorded
// as a RequestFactory data_fetch site.
func TestGWTRPC_RequestFire_Issue3190(t *testing.T) {
	source := `
package com.example.client;

import com.google.web.bindery.requestfactory.shared.Receiver;

public class EmployeePresenter {
    public void load(EmployeeRequest req) {
        req.findEmployee(42L).fire(new Receiver<EmployeeProxy>() {
            public void onSuccess(EmployeeProxy response) {}
        });
    }
}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "gwt",
		FilePath: "EmployeePresenter.java",
	})

	if !gwtHasProvenance(r, "INFERRED_FROM_GWT_REQUEST_FIRE") {
		t.Errorf("[#3190 data_fetching] expected Request.fire data_fetch op, got none")
	}
}

// TestGWTRPC_IgnoresNonGWTFramework_Issue3190 proves the framework gate works:
// the extractor must not fire on a non-GWT framework even with matching syntax.
func TestGWTRPC_IgnoresNonGWTFramework_Issue3190(t *testing.T) {
	source := `
@RemoteServiceRelativePath("greet")
public interface GreetingService extends RemoteService {}
`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "java", Framework: "spring_boot",
		FilePath: "GreetingService.java",
	})
	if len(r.Entities) != 0 {
		t.Errorf("[#3190 gate] extractor should not fire on spring_boot framework, got %d entities", len(r.Entities))
	}
}

// TestGWTRPC_IgnoresNonJava_Issue3190 proves the language gate works.
func TestGWTRPC_IgnoresNonJava_Issue3190(t *testing.T) {
	source := `GWT.create(GreetingService.class)`
	r := ExtractGWTDataFetching(PatternContext{
		Source: source, Language: "javascript", Framework: "gwt",
		FilePath: "x.js",
	})
	if len(r.Entities) != 0 {
		t.Errorf("[#3190 gate] extractor should not fire on non-java language")
	}
}
