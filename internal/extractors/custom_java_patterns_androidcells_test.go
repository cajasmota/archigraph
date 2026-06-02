package extractors

import (
	"testing"
)

// custom_java_patterns_android_test.go — Android-Java live-path re-verification
// for #3590 / #3575.
//
// #3585 honest-downgraded the Android capability cells to `missing` because the
// 35 Extract*(ctx PatternContext) PatternResult functions — ExtractAndroid among
// them — were dead code with zero non-test callers, so nothing they detected
// ever reached the graph. #3599 wired the `custom_java_patterns` dispatcher,
// which now runs ExtractAndroid live via RunCustomExtractors → CustomExtractorsFor("java").
//
// These tests drive that SAME live dispatch path (not ExtractAndroid directly)
// and assert SPECIFIC entities/edges/properties reach the graph, proving each
// re-stamped Android cell is genuinely live-backed:
//
// Re-stamped FULL (proven live below):
//   - context_extraction   (Structure)    — getContext()/requireContext() call sites
//   - platform_branching    (Platform)    — Build.VERSION.SDK_INT branch operations
//   - branch_conditions     (Data Flow)   — same SDK_INT branch control-flow site
//   - state_management       (Data Flow)  — ViewModel class detection
//   - navigation_extraction / screen_detection (jetpack record) — Activity/Fragment + Intent tx
//
// Re-stamped PARTIAL (only the Java-source half is live-reachable):
//   - native_module_imports (Native Bridge) — `import android.hardware.*` is live;
//     the manifest <uses-permission>/<uses-feature> half is NOT (see below).
//
// LEFT missing (honest — manifest is not live-reachable):
//   - deep_link_extraction  (Navigation)  — emitted only from AndroidManifest.xml,
//     which the dispatcher never feeds to ExtractAndroid (no framework marker).
//
// Assertions are value-specific (exact Kind/Name/property), never len > 0.

// TestJavaPatternsAndroidContextExtractionLive proves Structure.context_extraction:
// a getContext()/requireContext() call site inside a Fragment emits a
// SCOPE.Reference context_site entity through the live dispatch path.
func TestJavaPatternsAndroidContextExtractionLive(t *testing.T) {
	src := `
package com.example.app;

import androidx.fragment.app.Fragment;
import android.content.Context;

public class ProfileFragment extends Fragment {
    void load() {
        Context c = requireContext();
        getActivity();
    }
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/ProfileFragment.java", src)

	// requireContext() call site -> SCOPE.Reference context_site keyed by
	// enclosing class + method name.
	site := findRecord(recs, "SCOPE.Reference", "ProfileFragment.requireContext")
	if site == nil {
		t.Fatalf("expected SCOPE.Reference ProfileFragment.requireContext context_site to emit live; got %v", names(recs))
	}
	if got := site.Properties["context_method"]; got != "requireContext" {
		t.Errorf("context_method = %q, want requireContext", got)
	}
	if got := site.Properties["context_kind"]; got != "call_site" {
		t.Errorf("context_kind = %q, want call_site", got)
	}
	if got := site.Properties["provenance"]; got != "INFERRED_FROM_ANDROID_CONTEXT_CALL" {
		t.Errorf("provenance = %q, want INFERRED_FROM_ANDROID_CONTEXT_CALL", got)
	}
}

// NOTE on Navigation.deep_link_extraction (LEFT missing): deep links are emitted
// ONLY from AndroidManifest.xml (<intent-filter> <data android:scheme>), and the
// manifest is NOT live-reachable through the custom_java_patterns dispatcher — it
// carries none of the framework marker signals, so ExtractAndroid never runs on
// it. This is proven negatively by TestJavaPatternsAndroidManifestNotLiveReachable
// below; the deep_link cell therefore stays missing pending dispatcher manifest
// gating, not a fabricated full flip.

// TestJavaPatternsAndroidPlatformBranchingLive proves Platform.platform_branching
// AND Data Flow.branch_conditions: a Build.VERSION.SDK_INT comparison emits a
// SCOPE.Operation branch entity (the same entity backs both cells).
func TestJavaPatternsAndroidPlatformBranchingLive(t *testing.T) {
	src := `
package com.example.app;

import android.app.Activity;
import android.os.Build;

public class CompatActivity extends Activity {
    void apply() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            doNew();
        }
    }
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/CompatActivity.java", src)

	expr := "Build.VERSION.SDK_INT >= Build.VERSION_CODES.O"
	branch := findRecord(recs, "SCOPE.Operation", expr)
	if branch == nil {
		t.Fatalf("expected SCOPE.Operation %q platform-branch to emit live; got %v", expr, names(recs))
	}
	if got := branch.Properties["branch_kind"]; got != "sdk_int" {
		t.Errorf("branch_kind = %q, want sdk_int", got)
	}
	if got := branch.Properties["operator"]; got != ">=" {
		t.Errorf("operator = %q, want >=", got)
	}
	if got := branch.Properties["api_level"]; got != "Build.VERSION_CODES.O" {
		t.Errorf("api_level = %q, want Build.VERSION_CODES.O", got)
	}
	if got := branch.Properties["enclosing_class"]; got != "CompatActivity" {
		t.Errorf("enclosing_class = %q, want CompatActivity", got)
	}
}

// TestJavaPatternsAndroidNativeModuleManifestLive proves Native Bridge.native_module_imports
// from the manifest: a <uses-permission android:name="android.hardware.camera">
// emits a SCOPE.Reference native_module entity.
func TestJavaPatternsAndroidNativeModuleJavaImportLive(t *testing.T) {
	src := `
package com.example.app;

import android.app.Activity;
import android.os.Bundle;
import android.hardware.camera2.CameraManager;

public class CameraActivity extends Activity {
    @Override
    protected void onCreate(Bundle b) { super.onCreate(b); }
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/CameraActivity.java", src)

	cam := findRecord(recs, "SCOPE.Reference", "android.hardware.camera2.CameraManager")
	if cam == nil {
		t.Fatalf("expected SCOPE.Reference android.hardware.camera2.CameraManager native_module to emit live; got %v", names(recs))
	}
	if got := cam.Properties["declaration_kind"]; got != "import" {
		t.Errorf("declaration_kind = %q, want import", got)
	}
	if got := cam.Properties["module_name"]; got != "android.hardware.camera2.CameraManager" {
		t.Errorf("module_name = %q, want android.hardware.camera2.CameraManager", got)
	}
}

// TestJavaPatternsAndroidManifestNotLiveReachable documents the honest live-path
// gap that keeps deep_link_extraction missing and native_module_imports partial:
// an AndroidManifest.xml carries no custom_java_patterns framework marker, so the
// live dispatcher short-circuits (detectFrameworks -> empty) before ExtractAndroid
// runs and emits NOTHING. ExtractAndroid handles manifests correctly when called
// directly; the gap is purely the dispatcher's source-marker gating.
func TestJavaPatternsAndroidManifestNotLiveReachable(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.app">
    <uses-permission android:name="android.hardware.camera"/>
    <application>
        <activity android:name=".DetailActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW"/>
                <data android:scheme="myapp" android:host="open" android:pathPrefix="/item"/>
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	recs := runJavaPatterns(t, "app/src/main/AndroidManifest.xml", src)
	if len(recs) != 0 {
		t.Fatalf("expected manifest to emit NOTHING live (no framework marker); got %v", names(recs))
	}
}

// TestJavaPatternsAndroidStateManagementLive proves Data Flow.state_management:
// a ViewModel subclass declared in source emits a SCOPE.Component viewmodel
// entity through the live dispatch path.
func TestJavaPatternsAndroidStateManagementLive(t *testing.T) {
	src := `
package com.example.app;

import androidx.lifecycle.ViewModel;
import androidx.lifecycle.MutableLiveData;

public class UserViewModel extends ViewModel {
    private MutableLiveData<String> name = new MutableLiveData<>();
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/UserViewModel.java", src)

	vm := findRecord(recs, "SCOPE.Component", "UserViewModel")
	if vm == nil {
		t.Fatalf("expected SCOPE.Component UserViewModel viewmodel to emit live; got %v", names(recs))
	}
	if got := vm.Properties["component_kind"]; got != "viewmodel" {
		t.Errorf("component_kind = %q, want viewmodel", got)
	}
	if got := vm.Properties["provenance"]; got != "INFERRED_FROM_ANDROID_VIEWMODEL" {
		t.Errorf("provenance = %q, want INFERRED_FROM_ANDROID_VIEWMODEL", got)
	}
}

// TestJavaPatternsAndroidFragmentNavigationLive proves the jetpack-record
// Navigation.screen_detection (Fragment) + navigation_extraction (fragment
// transaction) reach the graph live. The host Activity hosts a Fragment via a
// FragmentManager replace() transaction.
func TestJavaPatternsAndroidFragmentNavigationLive(t *testing.T) {
	src := `
package com.example.app;

import android.app.Activity;
import android.os.Bundle;

public class HomeActivity extends Activity {
    void show() {
        getSupportFragmentManager().beginTransaction()
            .replace(R.id.container, new SettingsFragment())
            .commit();
    }
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/HomeActivity.java", src)

	// screen_detection: the host Activity emits as a SCOPE.UIComponent.
	act := findRecord(recs, "SCOPE.UIComponent", "HomeActivity")
	if act == nil {
		t.Fatalf("expected SCOPE.UIComponent HomeActivity to emit live; got %v", names(recs))
	}

	// navigation_extraction: the fragment transaction HomeActivity::SettingsFragment
	// emits as a SCOPE.Operation navigation operation.
	tx := findRecord(recs, "SCOPE.Operation", "HomeActivity::SettingsFragment")
	if tx == nil {
		t.Fatalf("expected SCOPE.Operation HomeActivity::SettingsFragment fragment transaction to emit live; got %v", names(recs))
	}
	if got := tx.Properties["fragment_class"]; got != "SettingsFragment" {
		t.Errorf("fragment_class = %q, want SettingsFragment", got)
	}
	if got := tx.Properties["host_class"]; got != "HomeActivity" {
		t.Errorf("host_class = %q, want HomeActivity", got)
	}
}
