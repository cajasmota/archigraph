package extractors

import (
	"sort"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
)

// TestEveryRegisteredCustomKeyDispatches is the dispatch-parity guard.
//
// Background (CORE #3587 / #3592): custom framework extractors register into
// the global registry under a language-encoding key prefix and are selected at
// runtime by CustomExtractorsFor via strings.HasPrefix against the prefix in
// customPrefixForLanguage. Their unit tests call .Extract() directly, so an
// extractor registered under a prefix that no language maps to (e.g. the bare
// `ruby_*` / `php_*` stems the Ruby and PHP framework extractors used to use)
// passes every unit test yet is NEVER invoked by the live pipeline — silently
// producing zero entities in production.
//
// This guard enumerates EVERY key in the real, fully-populated registry (the
// custom sub-packages are blank-imported by custom_registry.go) and asserts
// that every key which looks like a custom/framework extractor key dispatches
// under exactly one mapped prefix. Any key that does not is reported as a
// FAILURE listing the orphaned key, so a future mis-registration is caught at
// CI time instead of in production.
//
// It deliberately does NOT call cleanRegistry: the assertion is against the
// production registry contents, not a synthetic fixture set.
//
// Pre-fix, this test would have FAILED listing all 20 Ruby+PHP orphans (and it
// also covers the Lua framework keys — #3548 — which dispatch under the bare
// `lua_` prefix that customPrefixForLanguage maps explicitly).
func TestEveryRegisteredCustomKeyDispatches(t *testing.T) {
	// All dispatch prefixes that customPrefixForLanguage can select with.
	// Deduplicated because several languages share a prefix (e.g. js/ts).
	prefixSet := map[string]struct{}{}
	for _, prefix := range customPrefixForLanguage {
		prefixSet[prefix] = struct{}{}
	}
	prefixes := make([]string, 0, len(prefixSet))
	for p := range prefixSet {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	// dispatches reports whether a key would be selected by CustomExtractorsFor
	// for at least one language: it must HasPrefix some mapped prefix AND be
	// strictly longer than that prefix (mirroring the len(k) > len(prefix) guard
	// in CustomExtractorsFor, which excludes the bare base-language key).
	dispatches := func(key string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(key, p) && len(key) > len(p) {
				return true
			}
		}
		return false
	}

	// Authoritative classification of every registered key, derived from the
	// registry itself rather than a hand-maintained prefix list:
	//
	//   - A BASE language extractor registers under a bare language identifier
	//     (e.g. "go", "ruby", "php", "python", "lua", "typescript"). These are
	//     intentionally NON-dispatching and must be excluded.
	//   - A FRAMEWORK / custom extractor registers under "<baseLang>_<suffix>"
	//     (e.g. "custom_php_pest_test" under base "custom"? no — under a
	//     language stem). Concretely: its key starts with some *other*
	//     registered base key followed by "_". Every such key MUST dispatch.
	//
	// This catches the exact bug class of #3587/#3592 (Ruby/PHP framework
	// extractors that registered under bare "ruby_*"/"php_*" stems — which
	// start with the registered base keys "ruby"/"php" + "_" — yet matched no
	// dispatch prefix) AND #3548 (Lua), independent of the prefix-mapping
	// details. Pre-fix it would report all 20 orphans.
	registered := map[string]bool{}
	for _, k := range extractor.List() {
		registered[k] = true
	}

	// Known legitimate non-dispatching compound base extractors: keys of the
	// form "<baseLang>_<suffix>" that are themselves base/manifest extractors
	// (not framework dispatch targets). swift_package is the package-manifest
	// extractor, registered alongside the bare "swift" grammar extractor.
	nonDispatchingBaseAllow := map[string]bool{
		"swift_package": true,
	}

	// isCustomLooking reports whether key is a framework/custom extractor that
	// must dispatch: it begins with some *other* registered base key + "_" and
	// is not an allow-listed compound base extractor.
	isCustomLooking := func(key string) bool {
		if nonDispatchingBaseAllow[key] {
			return false
		}
		for base := range registered {
			if base != key && strings.HasPrefix(key, base+"_") {
				return true
			}
		}
		return false
	}

	var orphans []string
	for _, key := range extractor.List() {
		if isCustomLooking(key) && !dispatches(key) {
			orphans = append(orphans, key)
		}
	}
	sort.Strings(orphans)

	if len(orphans) > 0 {
		t.Fatalf("dispatch-parity violation: %d custom extractor key(s) register "+
			"under a prefix no language maps to in customPrefixForLanguage and "+
			"are therefore NEVER invoked by the live pipeline (unit tests calling "+
			".Extract() directly do not catch this):\n  %s\n"+
			"Fix: either add the prefix to customPrefixForLanguage or rename the "+
			"key to an existing mapped prefix (e.g. custom_<lang>_*).",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}

// TestRubyAndPhpOrphansNowDispatch is a focused regression assertion for the
// exact 20 keys fixed in #3587 / #3592: every previously-orphaned Ruby and PHP
// framework extractor must now be selected by CustomExtractorsFor for its
// language. This pins the fix so a future rename cannot silently re-orphan them.
func TestRubyAndPhpOrphansNowDispatch(t *testing.T) {
	cases := map[string][]string{
		"ruby": {
			"custom_ruby_auth",
			"custom_ruby_cuba_routing",
			"custom_ruby_dry_types",
			"custom_ruby_grape_deep",
			"custom_ruby_middleware",
			"custom_ruby_observability",
			"custom_ruby_routes",
			"custom_ruby_driver_schema",
			"custom_ruby_sinatra_deep",
			"custom_ruby_validation",
		},
		"php": {
			"custom_php_sql_driver_schema",
			"custom_php_obs_laravel_symfony",
			"custom_php_doctrine_orm_data",
			"custom_php_eloquent_orm_data",
			"custom_php_cycleorm_data",
			"custom_php_propel_orm_data",
			"custom_php_redbeanphp_data",
			"custom_php_behat_test",
			"custom_php_codeception_test",
			"custom_php_pest_test",
		},
	}

	for lang, wantKeys := range cases {
		selected := map[string]bool{}
		for _, e := range CustomExtractorsFor(lang) {
			selected[e.Language()] = true
		}
		for _, k := range wantKeys {
			if !selected[k] {
				t.Errorf("%s: previously-orphaned extractor %q is not selected by "+
					"CustomExtractorsFor(%q) — it would never run live", lang, k, lang)
			}
		}
	}
}
