package java

import (
	"testing"
)

// ============================================================================
// Issue #3256: Android context_extraction + deep_link_extraction
// ============================================================================
//
// context_extraction: Android Context-acquiring call sites and Context
// parameter patterns extracted from Java source files.
//
// deep_link_extraction: <intent-filter> blocks in AndroidManifest.xml that
// contain <data android:scheme="..."> deep-link URI templates.
//
// Registry targets (both flipped to partial):
//   lang.java.framework.android-sdk     Structure/context_extraction
//   lang.java.framework.android-sdk     Navigation/deep_link_extraction
//   lang.java.framework.android-jetpack Structure/context_extraction
//   lang.java.framework.android-jetpack Navigation/deep_link_extraction
//
// Cite: internal/custom/java/android.go

// ── context_extraction ────────────────────────────────────────────────────────

// androidContextCallFixture contains representative Context call sites found
// in a typical Android Activity / Fragment / Service.
const androidContextCallFixture = `package com.example.app;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;

public class MainFragment extends Fragment {

    private ImageLoader loader;

    @Override
    public void onAttach(Context context) {
        super.onAttach(context);
        this.loader = new ImageLoader(context);
    }

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        Context ctx = getContext();
        loader.load(ctx, "https://example.com/img.png");
    }

    public void navigateToDetail(long itemId) {
        Context appCtx = getApplicationContext();
        Intent intent = new Intent(appCtx, DetailActivity.class);
        startActivity(intent);
    }

    public void onResume() {
        super.onResume();
        Context mContext = requireContext();
        Toast.makeText(mContext, "Resumed", Toast.LENGTH_SHORT).show();
    }
}
`

// TestAndroid_ContextExtraction_CallSites_Issue3256 proves that
// getContext(), getApplicationContext(), and requireContext() call sites are
// extracted as SCOPE.Reference context_site entities.
//
// Registry target: lang.java.framework.android-sdk Structure/context_extraction → partial
// Registry target: lang.java.framework.android-jetpack Structure/context_extraction → partial
func TestAndroid_ContextExtraction_CallSites_Issue3256(t *testing.T) {
	for _, fw := range []string{"android", "android_sdk", "android_jetpack"} {
		r := ExtractAndroid(PatternContext{
			Source:    androidContextCallFixture,
			Language:  "java",
			Framework: fw,
			FilePath:  "MainFragment.java",
		})

		contextSites := make(map[string]bool)
		for _, e := range r.Entities {
			if e.Subtype == "context_site" && e.Provenance == "INFERRED_FROM_ANDROID_CONTEXT_CALL" {
				if e.Kind != "SCOPE.Reference" {
					t.Errorf("[#3256 context_extraction fw=%s] expected SCOPE.Reference, got %s", fw, e.Kind)
				}
				if e.Properties["framework"] != "android" {
					t.Errorf("[#3256 context_extraction fw=%s] expected framework=android, got %v", fw, e.Properties["framework"])
				}
				if method, ok := e.Properties["context_method"].(string); ok {
					contextSites[method] = true
				}
			}
		}

		for _, want := range []string{"getContext", "getApplicationContext", "requireContext"} {
			if !contextSites[want] {
				t.Errorf("[#3256 context_extraction fw=%s] expected context call %q, got %v", fw, want, contextSites)
			}
		}
		if len(contextSites) == 0 {
			t.Errorf("[#3256 context_extraction fw=%s] no context_site entities extracted", fw)
		}
	}
}

// TestAndroid_ContextExtraction_ContextParams_Issue3256 proves that Context
// parameters (well-known variable names) are extracted as context_site entities
// with context_kind=parameter.
func TestAndroid_ContextExtraction_ContextParams_Issue3256(t *testing.T) {
	src := `package com.example;

import android.content.Context;

public class HelperUtil {
    private final Context mContext;

    public HelperUtil(Context context) {
        this.mContext = context;
    }

    public void doWork(Context appContext) {
        // uses applicationContext
    }
}
`
	r := ExtractAndroid(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "android_sdk",
		FilePath:  "HelperUtil.java",
	})

	paramSites := 0
	for _, e := range r.Entities {
		if e.Subtype == "context_site" && e.Provenance == "INFERRED_FROM_ANDROID_CONTEXT_PARAM" {
			paramSites++
			if e.Properties["context_kind"] != "parameter" {
				t.Errorf("[#3256 context_extraction] expected context_kind=parameter, got %v", e.Properties["context_kind"])
			}
		}
	}
	if paramSites == 0 {
		t.Errorf("[#3256 context_extraction] expected at least one Context parameter entity")
	}
}

// TestAndroid_ContextExtraction_ManifestSkipped_Issue3256 proves that
// context extraction does NOT fire on AndroidManifest.xml (XML files
// should not be scanned for Java call sites).
func TestAndroid_ContextExtraction_ManifestSkipped_Issue3256(t *testing.T) {
	// The manifest fixture contains no context call sites — this is just a gate
	// check to ensure no false positives from manifest content.
	r := ExtractAndroid(PatternContext{
		Source:    androidManifestNativeFixture,
		Language:  "java",
		Framework: "android",
		FilePath:  "app/src/main/AndroidManifest.xml",
	})

	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_ANDROID_CONTEXT_CALL" ||
			e.Provenance == "INFERRED_FROM_ANDROID_CONTEXT_PARAM" {
			t.Errorf("[#3256 context_extraction] context extractor fired on manifest: %v", e)
		}
	}
}

// TestAndroid_ContextExtraction_NonAndroidFramework_Issue3256 proves the gate
// works — non-Android framework should yield no context entities.
func TestAndroid_ContextExtraction_NonAndroidFramework_Issue3256(t *testing.T) {
	r := ExtractAndroid(PatternContext{
		Source:    androidContextCallFixture,
		Language:  "java",
		Framework: "spring_boot",
		FilePath:  "MainFragment.java",
	})
	if len(r.Entities) != 0 {
		t.Errorf("[#3256 context_extraction] non-android framework should yield 0 entities, got %d", len(r.Entities))
	}
}

// ── deep_link_extraction ──────────────────────────────────────────────────────

// androidDeepLinkManifestFixture is a realistic AndroidManifest.xml fragment
// containing two deep-link intent-filters: one with scheme+host+pathPrefix,
// one with scheme only (minimal), plus a regular launcher intent-filter (no
// <data> scheme element — must not be emitted as a deep link).
const androidDeepLinkManifestFixture = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.app">
    <application>
        <activity android:name=".MainActivity">
            <!-- Launcher intent — NOT a deep link -->
            <intent-filter>
                <action android:name="android.intent.action.MAIN"/>
                <category android:name="android.intent.category.LAUNCHER"/>
            </intent-filter>

            <!-- Deep link: myapp://open/item -->
            <intent-filter>
                <action android:name="android.intent.action.VIEW"/>
                <category android:name="android.intent.category.DEFAULT"/>
                <category android:name="android.intent.category.BROWSABLE"/>
                <data android:scheme="myapp"
                      android:host="open"
                      android:pathPrefix="/item"/>
            </intent-filter>

            <!-- Deep link: https://www.example.com (App Link) -->
            <intent-filter android:autoVerify="true">
                <action android:name="android.intent.action.VIEW"/>
                <category android:name="android.intent.category.DEFAULT"/>
                <category android:name="android.intent.category.BROWSABLE"/>
                <data android:scheme="https"
                      android:host="www.example.com"/>
            </intent-filter>
        </activity>
    </application>
</manifest>
`

// TestAndroid_DeepLinkExtraction_IntentFilter_Issue3256 proves that
// <intent-filter> blocks containing <data android:scheme="..."> are extracted
// as SCOPE.Reference deep_link entities.
//
// Registry target: lang.java.framework.android-sdk Navigation/deep_link_extraction → partial
// Registry target: lang.java.framework.android-jetpack Navigation/deep_link_extraction → partial
func TestAndroid_DeepLinkExtraction_IntentFilter_Issue3256(t *testing.T) {
	for _, fw := range []string{"android", "android_sdk", "android_jetpack"} {
		r := ExtractAndroid(PatternContext{
			Source:    androidDeepLinkManifestFixture,
			Language:  "java",
			Framework: fw,
			FilePath:  "app/src/main/AndroidManifest.xml",
		})

		deepLinks := make(map[string]bool)
		for _, e := range r.Entities {
			if e.Provenance == "INFERRED_FROM_ANDROID_DEEP_LINK" {
				if e.Kind != "SCOPE.Reference" {
					t.Errorf("[#3256 deep_link fw=%s] expected SCOPE.Reference, got %s", fw, e.Kind)
				}
				if e.Subtype != "deep_link" {
					t.Errorf("[#3256 deep_link fw=%s] expected subtype=deep_link, got %s", fw, e.Subtype)
				}
				if e.Properties["framework"] != "android" {
					t.Errorf("[#3256 deep_link fw=%s] expected framework=android, got %v", fw, e.Properties["framework"])
				}
				deepLinks[e.Properties["scheme"].(string)] = true
			}
		}

		// Both myapp:// and https:// deep links must be found.
		for _, want := range []string{"myapp", "https"} {
			if !deepLinks[want] {
				t.Errorf("[#3256 deep_link fw=%s] expected scheme %q, got %v", fw, want, deepLinks)
			}
		}
		// Exactly 2 deep-link entities (the launcher filter has no scheme).
		if len(deepLinks) != 2 {
			t.Errorf("[#3256 deep_link fw=%s] expected 2 deep-link schemes, got %d: %v", fw, len(deepLinks), deepLinks)
		}
	}
}

// TestAndroid_DeepLinkExtraction_URITemplate_Issue3256 proves that the URI
// template (scheme://host/path) is correctly assembled from the manifest data.
func TestAndroid_DeepLinkExtraction_URITemplate_Issue3256(t *testing.T) {
	r := ExtractAndroid(PatternContext{
		Source:    androidDeepLinkManifestFixture,
		Language:  "java",
		Framework: "android_sdk",
		FilePath:  "app/src/main/AndroidManifest.xml",
	})

	urisSeen := make(map[string]bool)
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_ANDROID_DEEP_LINK" {
			uri, _ := e.Properties["uri"].(string)
			urisSeen[uri] = true
		}
	}

	// myapp://open/item (scheme + host + pathPrefix)
	if !urisSeen["myapp://open/item"] {
		t.Errorf("[#3256 deep_link] expected URI myapp://open/item, got %v", urisSeen)
	}
	// https://www.example.com (scheme + host, no path)
	if !urisSeen["https://www.example.com"] {
		t.Errorf("[#3256 deep_link] expected URI https://www.example.com, got %v", urisSeen)
	}
}

// TestAndroid_DeepLinkExtraction_SchemeProperties_Issue3256 proves that scheme
// and host properties are set on the deep_link entity.
func TestAndroid_DeepLinkExtraction_SchemeProperties_Issue3256(t *testing.T) {
	r := ExtractAndroid(PatternContext{
		Source:    androidDeepLinkManifestFixture,
		Language:  "java",
		Framework: "android",
		FilePath:  "app/src/main/AndroidManifest.xml",
	})

	for _, e := range r.Entities {
		if e.Provenance != "INFERRED_FROM_ANDROID_DEEP_LINK" {
			continue
		}
		scheme, _ := e.Properties["scheme"].(string)
		if scheme == "" {
			t.Errorf("[#3256 deep_link] expected non-empty scheme property on %v", e.Name)
		}
		if e.Properties["framework"] != "android" {
			t.Errorf("[#3256 deep_link] expected framework=android, got %v", e.Properties["framework"])
		}
	}
}

// TestAndroid_DeepLinkExtraction_JavaFileSkipped_Issue3256 proves that
// deep-link extraction does NOT fire on Java source files (only manifests).
func TestAndroid_DeepLinkExtraction_JavaFileSkipped_Issue3256(t *testing.T) {
	src := `package com.example;
// <intent-filter>  <data android:scheme="test"/>  </intent-filter>
// This is a comment, not XML — should not be parsed.
public class NavigationHelper {}
`
	r := ExtractAndroid(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "android",
		FilePath:  "NavigationHelper.java",
	})
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_ANDROID_DEEP_LINK" {
			t.Errorf("[#3256 deep_link] deep-link extractor fired on Java source: %v", e)
		}
	}
}

// TestAndroid_DeepLinkExtraction_MinimalSchemeOnly_Issue3256 proves that a
// deep-link with only a scheme (no host or path) is still extracted.
func TestAndroid_DeepLinkExtraction_MinimalSchemeOnly_Issue3256(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example">
    <application>
        <activity android:name=".SplashActivity">
            <intent-filter>
                <action android:name="android.intent.action.VIEW"/>
                <data android:scheme="splash"/>
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	r := ExtractAndroid(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "android_sdk",
		FilePath:  "AndroidManifest.xml",
	})

	found := false
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_ANDROID_DEEP_LINK" {
			scheme, _ := e.Properties["scheme"].(string)
			if scheme == "splash" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("[#3256 deep_link] expected deep-link with scheme=splash for minimal intent-filter")
	}
}
