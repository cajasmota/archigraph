package java

import "testing"

// ============================================================================
// CDI Interceptor extractor — MicroProfile gate tests (issue #3175)
//
// MicroProfile is a CDI-based Jakarta EE profile; its interceptor model is
// identical to Jakarta EE CDI.  Adding "microprofile" to cdiFrameworks makes
// ExtractCDIInterceptors fire for microprofile inputs.
//
// Proves all three AOP cells for lang.java.framework.microprofile:
//   - aspect_extraction    (@Interceptor class)
//   - advice_attribution   (@AroundInvoke method)
//   - pointcut_resolution  (@InterceptorBinding annotation declaration)
// ============================================================================

// TestCDI_MicroProfile_AspectExtraction_Issue3175 proves that an @Interceptor
// class is extracted as an aspect entity when framework="microprofile".
func TestCDI_MicroProfile_AspectExtraction_Issue3175(t *testing.T) {
	source := `
package com.example.mp;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundInvoke;
import jakarta.interceptor.InvocationContext;
import org.eclipse.microprofile.faulttolerance.Retry;

@Retry
@Interceptor
public class RetryInterceptor {

    @AroundInvoke
    public Object intercept(InvocationContext ctx) throws Exception {
        return ctx.proceed();
    }
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "microprofile",
		FilePath:  "RetryInterceptor.java",
	})

	// aspect_extraction
	asp, ok := cdiEntityByName(r, "aspect", "RetryInterceptor")
	if !ok {
		t.Fatalf("[#3175 aspect] expected aspect entity RetryInterceptor; got %v", entityNames(r.Entities))
	}
	if asp.Kind != "SCOPE.Pattern" {
		t.Errorf("[#3175 aspect] Kind = %q, want SCOPE.Pattern", asp.Kind)
	}
	if asp.Properties["kind"] != "cdi_interceptor" {
		t.Errorf("[#3175 aspect] kind property = %v, want cdi_interceptor", asp.Properties["kind"])
	}

	// advice_attribution
	adv, ok := cdiEntityByName(r, "advice", "RetryInterceptor.intercept")
	if !ok {
		t.Fatalf("[#3175 advice] expected advice entity RetryInterceptor.intercept; got %v", entityNames(r.Entities))
	}
	if adv.Properties["advice_type"] != "around_invoke" {
		t.Errorf("[#3175 advice] advice_type = %v, want around_invoke", adv.Properties["advice_type"])
	}

	// OWNS edge
	if !cdiHasRel(r, asp.Ref, adv.Ref, "OWNS") {
		t.Errorf("[#3175 owns] expected OWNS edge interceptor -> advice")
	}
}

// TestCDI_MicroProfile_PointcutResolution_Issue3175 proves that an
// @InterceptorBinding annotation declaration is extracted as a pointcut entity
// when framework="microprofile".
func TestCDI_MicroProfile_PointcutResolution_Issue3175(t *testing.T) {
	source := `
package com.example.mp;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import jakarta.interceptor.InterceptorBinding;

@InterceptorBinding
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
public @interface Metered {
}
`
	r := ExtractCDIInterceptors(PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "microprofile",
		FilePath:  "Metered.java",
	})

	// pointcut_resolution
	pc, ok := cdiEntityByName(r, "pointcut", "Metered")
	if !ok {
		t.Fatalf("[#3175 pointcut] expected pointcut entity Metered; got %v", entityNames(r.Entities))
	}
	if pc.Kind != "SCOPE.Pattern" {
		t.Errorf("[#3175 pointcut] Kind = %q, want SCOPE.Pattern", pc.Kind)
	}
	if pc.Properties["kind"] != "interceptor_binding" {
		t.Errorf("[#3175 pointcut] kind property = %v, want interceptor_binding", pc.Properties["kind"])
	}
}

// TestCDI_MicroProfile_GateVariants_Issue3175 verifies that both "micro-profile"
// and "micro_profile" aliases also trigger the extractor.
func TestCDI_MicroProfile_GateVariants_Issue3175(t *testing.T) {
	source := `
package com.example.mp;

import jakarta.interceptor.Interceptor;
import jakarta.interceptor.AroundInvoke;
import jakarta.interceptor.InvocationContext;

@Interceptor
public class TracingInterceptor {
    @AroundInvoke
    public Object trace(InvocationContext ctx) throws Exception {
        return ctx.proceed();
    }
}
`
	for _, fw := range []string{"micro-profile", "micro_profile"} {
		r := ExtractCDIInterceptors(PatternContext{
			Source:    source,
			Language:  "java",
			Framework: fw,
			FilePath:  "TracingInterceptor.java",
		})
		if _, ok := cdiEntityByName(r, "aspect", "TracingInterceptor"); !ok {
			t.Errorf("[#3175 gate-variant %q] expected aspect entity TracingInterceptor; got %v", fw, entityNames(r.Entities))
		}
	}
}
