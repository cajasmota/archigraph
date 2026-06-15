package vue_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// Issue #2876 — Vue Internals framework_specific group. One proving fixture per
// idiom family. The (A) recording cells (composition_api, options_api,
// sfc_block_extraction, provide_inject, props_emits_macros, pinia_store) and the
// newly implemented (B) cells (directive_recognition, slot_extraction) are all
// asserted against hand-written .vue fixtures so no cell is flipped without a
// proving fixture.

func loadVueFixture(t *testing.T, rel string) string {
	t.Helper()
	// Test runs from internal/extractors/vue/; fixtures live in the sibling
	// javascript extractor testdata tree (per the issue's fixture path).
	p := filepath.Join("..", "javascript", "testdata", "vue_internals", rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixture %s: %v", p, err)
	}
	return string(b)
}

func hasEntity(ents []types.EntityRecord, subtype, name string) bool {
	for i := range ents {
		if ents[i].Subtype == subtype && ents[i].Name == name {
			return true
		}
	}
	return false
}

func hasSubtype(ents []types.EntityRecord, subtype string) bool {
	for i := range ents {
		if ents[i].Subtype == subtype {
			return true
		}
	}
	return false
}

// TestIssue2876_VueInternals_ScriptSetup proves the <script setup> idiom cells:
// composition_api, props_emits_macros, provide_inject, pinia_store,
// sfc_block_extraction, directive_recognition, slot_extraction.
func TestIssue2876_VueInternals_ScriptSetup(t *testing.T) {
	src := loadVueFixture(t, "Comp.vue")
	ents := extract(t, "src/components/Comp.vue", src)

	comp := findByName(ents, "Comp")
	if comp == nil {
		t.Fatalf("Comp component not extracted; %s", dump(ents))
	}

	// composition_api: ref/computed/watch/provide/inject → CALLS edges.
	for _, callee := range []string{"ref", "computed", "watch", "provide", "inject"} {
		if !relTarget(comp, "CALLS", callee) {
			t.Errorf("composition_api: missing CALLS %s; %s", callee, dump(ents))
		}
	}

	// props_emits_macros: defineProps / defineEmits / defineExpose operations.
	if !hasEntity(ents, "define_props", "defineProps") {
		t.Errorf("props_emits_macros: missing defineProps; %s", dump(ents))
	}
	if !hasEntity(ents, "define_emits", "defineEmits") {
		t.Errorf("props_emits_macros: missing defineEmits")
	}
	if !hasEntity(ents, "define_expose", "defineExpose") {
		t.Errorf("props_emits_macros: missing defineExpose")
	}

	// provide_inject: provider/consumer context operations.
	if !hasEntity(ents, "provide_context", "provider:theme") {
		t.Errorf("provide_inject: missing provider:theme; %s", dump(ents))
	}
	if !hasEntity(ents, "inject_context", "consumer:currentUser") {
		t.Errorf("provide_inject: missing consumer:currentUser")
	}

	// pinia_store: defineStore import + useCounterStore() → state_store.
	if !hasEntity(ents, "state_store", "useCounterStore") {
		t.Errorf("pinia_store: missing state_store useCounterStore; %s", dump(ents))
	}

	// sfc_block_extraction: a vue_component entity is produced from the SFC and
	// the <script setup> block was parsed (proven by the macro/CALLS extraction
	// above which only runs on a successfully split <script> block). The <style>
	// block is intentionally ignored (scoped_style_extraction = N/A).
	if comp.Subtype != "vue_component" {
		t.Errorf("sfc_block_extraction: component subtype = %q, want vue_component", comp.Subtype)
	}

	// directive_recognition (B): v-model, v-for, v-bind (:max shorthand),
	// v-on (@input shorthand).
	dirs := map[string]bool{}
	for i := range ents {
		if ents[i].Subtype == "directive" {
			dirs[ents[i].Properties["directive"]] = true
		}
	}
	for _, d := range []string{"v-model", "v-for", "v-bind", "v-on"} {
		if !dirs[d] {
			t.Errorf("directive_recognition: missing directive %s; dirs=%v; %s", d, dirs, dump(ents))
		}
	}

	// slot_extraction (B): outlet slots (default + named "title") and content
	// slots (#header, v-slot:footer).
	if !hasEntity(ents, "slot", "outlet_default") {
		t.Errorf("slot_extraction: missing default slot outlet; %s", dump(ents))
	}
	if !hasEntity(ents, "slot", "outlet_title") {
		t.Errorf("slot_extraction: missing named slot outlet 'title'")
	}
	if !hasEntity(ents, "slot", "content_header") {
		t.Errorf("slot_extraction: missing content slot 'header'")
	}
	if !hasEntity(ents, "slot", "content_footer") {
		t.Errorf("slot_extraction: missing content slot 'footer'")
	}

	// scoped_style_extraction (N/A): the <style scoped> block must NOT produce a
	// style entity — CSS is ignored by design (extractor.go package doc).
	if hasSubtype(ents, "style") {
		t.Errorf("scoped_style_extraction is N/A but a style entity was emitted; %s", dump(ents))
	}
}

// TestIssue2876_VueInternals_OptionsAPI proves the options_api cell.
func TestIssue2876_VueInternals_OptionsAPI(t *testing.T) {
	src := loadVueFixture(t, "OptionsComp.vue")
	ents := extract(t, "src/components/OptionsComp.vue", src)

	comp := findByName(ents, "OptionsComp")
	if comp == nil {
		t.Fatalf("OptionsComp not extracted; %s", dump(ents))
	}

	// options_api: methods declared in the methods: { … } block.
	if !hasEntity(ents, "method", "increment") {
		t.Errorf("options_api: missing method increment; %s", dump(ents))
	}
	if !hasEntity(ents, "method", "reset") {
		t.Errorf("options_api: missing method reset")
	}
}
