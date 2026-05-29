package java

import "testing"

// ============================================================================
// CDI Interceptor / AOP extractor tests (issue #3082)
// Cells: advice_attribution, aspect_extraction, pointcut_resolution
// Frameworks: jakarta-ee, jaxrs, quarkus
// ============================================================================

// cdiEntityByName returns the first entity with the given subtype and name.
func cdiEntityByName(r PatternResult, subtype, name string) (SecondaryEntity, bool) {
	for _, e := range r.Entities {
		if e.Subtype == subtype && e.Name == name {
			return e, true
		}
	}
	return SecondaryEntity{}, false
}

// cdiHasRel reports whether a directed edge (src, tgt, kind) is present.
func cdiHasRel(r PatternResult, src, tgt, kind string) bool {
	for _, rel := range r.Relationships {
		if rel.SourceRef == src && rel.TargetRef == tgt && rel.RelationshipType == kind {
			return true
		}
	}
	return false
}

// ============================================================================
// TestCDI_JakartaEE_InterceptorClass_Issue3082
// Proves aspect_extraction for jakarta-ee: @Interceptor class emits an
// SCOPE.Pattern(subtype=aspect, kind=cdi_interceptor) entity.
// ============================================================================
func TestCDI_JakartaEE_InterceptorClass_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundInvoke;
import jakarta.interceptor.InvocationContext;

@Logged
@Interceptor
public class LoggingInterceptor {

    @AroundInvoke
    public Object intercept(InvocationContext ctx) throws Exception {
        System.out.println("Before: " + ctx.getMethod().getName());
        Object result = ctx.proceed();
        System.out.println("After: " + ctx.getMethod().getName());
        return result;
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jakarta_ee",
		FilePath:  "LoggingInterceptor.java",
	})

	// aspect_extraction: interceptor class entity.
	asp, ok := cdiEntityByName(r, "aspect", "LoggingInterceptor")
	if !ok {
		t.Fatalf("[#3082 aspect] expected aspect entity LoggingInterceptor; got %v", entityNames(r.Entities))
	}
	if asp.Kind != "SCOPE.Pattern" {
		t.Errorf("[#3082 aspect] Kind = %q, want SCOPE.Pattern", asp.Kind)
	}
	if asp.Properties["kind"] != "cdi_interceptor" {
		t.Errorf("[#3082 aspect] kind property = %v, want cdi_interceptor", asp.Properties["kind"])
	}
	if asp.Properties["framework"] != "jakarta_ee" {
		t.Errorf("[#3082 aspect] framework = %v, want jakarta_ee", asp.Properties["framework"])
	}

	// advice_attribution: @AroundInvoke method entity.
	adv, ok := cdiEntityByName(r, "advice", "LoggingInterceptor.intercept")
	if !ok {
		t.Fatalf("[#3082 advice] expected advice entity LoggingInterceptor.intercept; got %v", entityNames(r.Entities))
	}
	if adv.Properties["advice_type"] != "around_invoke" {
		t.Errorf("[#3082 advice] advice_type = %v, want around_invoke", adv.Properties["advice_type"])
	}
	if adv.Properties["aspect"] != "LoggingInterceptor" {
		t.Errorf("[#3082 advice] aspect = %v, want LoggingInterceptor", adv.Properties["aspect"])
	}
	if adv.Properties["framework"] != "jakarta_ee" {
		t.Errorf("[#3082 advice] framework = %v, want jakarta_ee", adv.Properties["framework"])
	}

	// OWNS edge: interceptor -> advice.
	if !cdiHasRel(r, asp.Ref, adv.Ref, "OWNS") {
		t.Errorf("[#3082 owns] expected OWNS edge interceptor -> advice")
	}
}

// ============================================================================
// TestCDI_JakartaEE_InterceptorBinding_Issue3082
// Proves pointcut_resolution for jakarta-ee: @InterceptorBinding annotation
// declaration emits a SCOPE.Pattern(subtype=pointcut, kind=interceptor_binding).
// ============================================================================
func TestCDI_JakartaEE_InterceptorBinding_Issue3082(t *testing.T) {
	source := `
package com.example;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import jakarta.interceptor.InterceptorBinding;

@InterceptorBinding
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
public @interface Logged {
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jakarta_ee",
		FilePath:  "Logged.java",
	})

	// pointcut_resolution: binding entity.
	pc, ok := cdiEntityByName(r, "pointcut", "Logged")
	if !ok {
		t.Fatalf("[#3082 pointcut] expected pointcut entity Logged; got %v", entityNames(r.Entities))
	}
	if pc.Kind != "SCOPE.Pattern" {
		t.Errorf("[#3082 pointcut] Kind = %q, want SCOPE.Pattern", pc.Kind)
	}
	if pc.Properties["kind"] != "interceptor_binding" {
		t.Errorf("[#3082 pointcut] kind property = %v, want interceptor_binding", pc.Properties["kind"])
	}
	if pc.Properties["binding"] != "Logged" {
		t.Errorf("[#3082 pointcut] binding = %v, want Logged", pc.Properties["binding"])
	}
	if pc.Properties["framework"] != "jakarta_ee" {
		t.Errorf("[#3082 pointcut] framework = %v, want jakarta_ee", pc.Properties["framework"])
	}
}

// ============================================================================
// TestCDI_JakartaEE_AroundConstruct_Issue3082
// Proves advice_attribution for @AroundConstruct in jakarta-ee.
// ============================================================================
func TestCDI_JakartaEE_AroundConstruct_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundConstruct;
import jakarta.interceptor.InvocationContext;

@Secured
@Interceptor
public class SecurityInterceptor {

    @AroundConstruct
    public void aroundConstruct(InvocationContext ctx) throws Exception {
        ctx.proceed();
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jakarta-ee",
		FilePath:  "SecurityInterceptor.java",
	})

	asp, ok := cdiEntityByName(r, "aspect", "SecurityInterceptor")
	if !ok {
		t.Fatalf("[#3082 aspect] expected SecurityInterceptor; got %v", entityNames(r.Entities))
	}

	adv, ok := cdiEntityByName(r, "advice", "SecurityInterceptor.aroundConstruct")
	if !ok {
		t.Fatalf("[#3082 advice] expected advice SecurityInterceptor.aroundConstruct; got %v", entityNames(r.Entities))
	}
	if adv.Properties["advice_type"] != "around_construct" {
		t.Errorf("[#3082 advice] advice_type = %v, want around_construct", adv.Properties["advice_type"])
	}
	if !cdiHasRel(r, asp.Ref, adv.Ref, "OWNS") {
		t.Errorf("[#3082 owns] expected OWNS edge SecurityInterceptor -> aroundConstruct")
	}
	// Framework alias normalises to jakarta_ee.
	if asp.Properties["framework"] != "jakarta_ee" {
		t.Errorf("[#3082 aspect] framework = %v, want jakarta_ee (normalised)", asp.Properties["framework"])
	}
}

// ============================================================================
// TestCDI_JAXRs_InterceptorClass_Issue3082
// Proves aspect_extraction and advice_attribution for the jaxrs framework.
// ============================================================================
func TestCDI_JAXRS_InterceptorClass_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundInvoke;
import jakarta.interceptor.InvocationContext;

@Audited
@Interceptor
public class AuditInterceptor {

    @AroundInvoke
    public Object audit(InvocationContext ctx) throws Exception {
        return ctx.proceed();
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jaxrs",
		FilePath:  "AuditInterceptor.java",
	})

	asp, ok := cdiEntityByName(r, "aspect", "AuditInterceptor")
	if !ok {
		t.Fatalf("[#3082 jaxrs aspect] expected AuditInterceptor; got %v", entityNames(r.Entities))
	}
	if asp.Properties["framework"] != "jaxrs" {
		t.Errorf("[#3082 jaxrs aspect] framework = %v, want jaxrs", asp.Properties["framework"])
	}

	adv, ok := cdiEntityByName(r, "advice", "AuditInterceptor.audit")
	if !ok {
		t.Fatalf("[#3082 jaxrs advice] expected advice AuditInterceptor.audit; got %v", entityNames(r.Entities))
	}
	if adv.Properties["advice_type"] != "around_invoke" {
		t.Errorf("[#3082 jaxrs advice] advice_type = %v, want around_invoke", adv.Properties["advice_type"])
	}
	if !cdiHasRel(r, asp.Ref, adv.Ref, "OWNS") {
		t.Errorf("[#3082 jaxrs owns] expected OWNS edge")
	}
}

// ============================================================================
// TestCDI_JAXRS_InterceptorBinding_Issue3082
// Proves pointcut_resolution for jaxrs framework.
// ============================================================================
func TestCDI_JAXRS_InterceptorBinding_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.InterceptorBinding;
import java.lang.annotation.*;

@InterceptorBinding
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
public @interface Audited {
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jax-rs",
		FilePath:  "Audited.java",
	})

	pc, ok := cdiEntityByName(r, "pointcut", "Audited")
	if !ok {
		t.Fatalf("[#3082 jaxrs pointcut] expected Audited; got %v", entityNames(r.Entities))
	}
	if pc.Properties["framework"] != "jaxrs" {
		t.Errorf("[#3082 jaxrs pointcut] framework = %v, want jaxrs (normalised)", pc.Properties["framework"])
	}
}

// ============================================================================
// TestCDI_Quarkus_InterceptorClass_Issue3082
// Proves aspect_extraction and advice_attribution for quarkus.
// ============================================================================
func TestCDI_Quarkus_InterceptorClass_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundInvoke;
import jakarta.interceptor.InvocationContext;

@Transactional
@Interceptor
public class TransactionInterceptor {

    @AroundInvoke
    public Object manageTransaction(InvocationContext ctx) throws Exception {
        return ctx.proceed();
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "quarkus",
		FilePath:  "TransactionInterceptor.java",
	})

	asp, ok := cdiEntityByName(r, "aspect", "TransactionInterceptor")
	if !ok {
		t.Fatalf("[#3082 quarkus aspect] expected TransactionInterceptor; got %v", entityNames(r.Entities))
	}
	if asp.Properties["framework"] != "quarkus" {
		t.Errorf("[#3082 quarkus aspect] framework = %v, want quarkus", asp.Properties["framework"])
	}

	adv, ok := cdiEntityByName(r, "advice", "TransactionInterceptor.manageTransaction")
	if !ok {
		t.Fatalf("[#3082 quarkus advice] expected advice TransactionInterceptor.manageTransaction; got %v", entityNames(r.Entities))
	}
	if adv.Properties["advice_type"] != "around_invoke" {
		t.Errorf("[#3082 quarkus advice] advice_type = %v, want around_invoke", adv.Properties["advice_type"])
	}
	if !cdiHasRel(r, asp.Ref, adv.Ref, "OWNS") {
		t.Errorf("[#3082 quarkus owns] expected OWNS edge")
	}
}

// ============================================================================
// TestCDI_Quarkus_InterceptorBinding_Issue3082
// Proves pointcut_resolution for quarkus: @InterceptorBinding detection.
// ============================================================================
func TestCDI_Quarkus_InterceptorBinding_Issue3082(t *testing.T) {
	source := `
package com.example;

import jakarta.interceptor.InterceptorBinding;
import java.lang.annotation.*;

@InterceptorBinding
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
public @interface Transactional {
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "quarkus",
		FilePath:  "Transactional.java",
	})

	pc, ok := cdiEntityByName(r, "pointcut", "Transactional")
	if !ok {
		t.Fatalf("[#3082 quarkus pointcut] expected Transactional; got %v", entityNames(r.Entities))
	}
	if pc.Kind != "SCOPE.Pattern" {
		t.Errorf("[#3082 quarkus pointcut] Kind = %q, want SCOPE.Pattern", pc.Kind)
	}
	if pc.Properties["kind"] != "interceptor_binding" {
		t.Errorf("[#3082 quarkus pointcut] kind property = %v, want interceptor_binding", pc.Properties["kind"])
	}
	if pc.Properties["framework"] != "quarkus" {
		t.Errorf("[#3082 quarkus pointcut] framework = %v, want quarkus", pc.Properties["framework"])
	}
}

// ============================================================================
// TestCDI_FrameworkGating_Issue3082
// Proves the extractor is gated correctly: runs for jakarta-ee/jaxrs/quarkus
// and no-ops for spring_boot, micronaut, python, etc.
// ============================================================================
func TestCDI_FrameworkGating_Issue3082(t *testing.T) {
	source := `
@Auditing
@Interceptor
public class AuditInterceptor {
    @AroundInvoke
    public Object audit(InvocationContext ctx) throws Exception { return ctx.proceed(); }
}
`
	activeFrameworks := []string{
		"jakarta_ee", "jakarta-ee", "jakartaee",
		"jaxrs", "jax-rs",
		"quarkus",
	}
	for _, fw := range activeFrameworks {
		r := ExtractCDIInterceptors(PatternContext{Source: source, Language: "java", Framework: fw, FilePath: "A.java"})
		if len(r.Entities) == 0 {
			t.Errorf("[#3082 gating] framework %q expected CDI entities, got none", fw)
		}
	}

	inactiveFrameworks := []string{"spring_boot", "spring-boot", "spring_webflux", "micronaut", "django"}
	for _, fw := range inactiveFrameworks {
		r := ExtractCDIInterceptors(PatternContext{Source: source, Language: "java", Framework: fw, FilePath: "A.java"})
		if len(r.Entities) != 0 {
			t.Errorf("[#3082 gating] framework %q should no-op, got %d entities", fw, len(r.Entities))
		}
	}

	// Non-java language must no-op even with a valid framework.
	r := ExtractCDIInterceptors(PatternContext{Source: source, Language: "python", Framework: "jakarta_ee", FilePath: "A.py"})
	if len(r.Entities) != 0 {
		t.Errorf("[#3082 gating] non-java should no-op, got %d entities", len(r.Entities))
	}
}

// ============================================================================
// TestCDI_NonInterceptorFile_Issue3082
// Proves that @AroundInvoke outside an @Interceptor class produces no advice
// entities (guards against phantom advice emission).
// ============================================================================
func TestCDI_NonInterceptorFile_Issue3082(t *testing.T) {
	source := `
package com.example;

// A plain bean without the interceptor annotation.
public class PlainBean {
    @AroundInvoke
    public Object notReallyAdvice(InvocationContext ctx) throws Exception {
        return ctx.proceed();
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "jakarta_ee",
		FilePath:  "PlainBean.java",
	})
	for _, e := range r.Entities {
		if e.Subtype == "advice" {
			t.Errorf("[#3082 non-interceptor] expected no advice entities, got %q", e.Name)
		}
	}
	// No aspect entity either.
	for _, e := range r.Entities {
		if e.Subtype == "aspect" {
			t.Errorf("[#3082 non-interceptor] expected no aspect entities, got %q", e.Name)
		}
	}
}
