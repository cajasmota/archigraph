package resolve

import "testing"

// TestDynamicPatterns_Groovy covers the groovyDynamicPatterns catalog for
// Groovy, Grails, and Gradle.
func TestDynamicPatterns_Groovy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		// 1. Grails controller DSL.
		{"groovy_grails_findResource", "groovy", `findResource`, true},
		{"groovy_grails_bindData", "groovy", `bindData`, true},
		{"groovy_grails_respond", "groovy", `respond`, true},
		{"groovy_grails_render", "groovy", `render`, true},
		{"groovy_grails_redirect", "groovy", `redirect`, true},
		{"groovy_grails_forward", "groovy", `forward`, true},
		{"groovy_grails_chain", "groovy", `chain`, true},
		{"groovy_grails_withFormat", "groovy", `withFormat`, true},
		{"groovy_grails_withTransaction", "groovy", `withTransaction`, true},
		{"groovy_grails_setRollbackOnly", "groovy", `setRollbackOnly`, true},
		// 2. GORM dynamic finders / persistence DSL.
		{"groovy_gorm_save", "groovy", `save`, true},
		{"groovy_gorm_delete", "groovy", `delete`, true},
		{"groovy_gorm_reload", "groovy", `reload`, true},
		{"groovy_gorm_refresh", "groovy", `refresh`, true},
		{"groovy_gorm_count", "groovy", `count`, true},
		{"groovy_gorm_findAll", "groovy", `findAll`, true},
		{"groovy_gorm_list", "groovy", `list`, true},
		{"groovy_gorm_exists", "groovy", `exists`, true},
		{"groovy_gorm_withCriteria", "groovy", `withCriteria`, true},
		{"groovy_gorm_createCriteria", "groovy", `createCriteria`, true},
		{"groovy_gorm_findByEmail", "groovy", `findByEmail`, true},
		{"groovy_gorm_findByStatus", "groovy", `findByStatus`, true},
		{"groovy_gorm_findAllByActive", "groovy", `findAllByActive`, true},
		{"groovy_gorm_countByRole", "groovy", `countByRole`, true},
		{"groovy_gorm_getByName", "groovy", `getByName`, true},
		{"groovy_gorm_validate", "groovy", `validate`, true},
		// 3. Spock test DSL.
		{"groovy_spock_thrown", "groovy", `thrown`, true},
		{"groovy_spock_notThrown", "groovy", `notThrown`, true},
		{"groovy_spock_old", "groovy", `old`, true},
		{"groovy_spock_Mock", "groovy", `Mock`, true},
		{"groovy_spock_Stub", "groovy", `Stub`, true},
		{"groovy_spock_Spy", "groovy", `Spy`, true},
		{"groovy_spock_mockDomains", "groovy", `mockDomains`, true},
		{"groovy_spock_interaction", "groovy", `interaction`, true},
		{"groovy_spock_noExceptionThrown", "groovy", `noExceptionThrown`, true},
		// 4. Gradle build script DSL.
		{"groovy_gradle_doLast", "groovy", `doLast`, true},
		{"groovy_gradle_doFirst", "groovy", `doFirst`, true},
		{"groovy_gradle_dependsOn", "groovy", `dependsOn`, true},
		{"groovy_gradle_from", "groovy", `from`, true},
		{"groovy_gradle_into", "groovy", `into`, true},
		{"groovy_gradle_repositories", "groovy", `repositories`, true},
		{"groovy_gradle_dependencies", "groovy", `dependencies`, true},
		{"groovy_gradle_implementation", "groovy", `implementation`, true},
		{"groovy_gradle_testImplementation", "groovy", `testImplementation`, true},
		{"groovy_gradle_mavenCentral", "groovy", `mavenCentral`, true},
		{"groovy_gradle_buildscript", "groovy", `buildscript`, true},
		{"groovy_gradle_allprojects", "groovy", `allprojects`, true},
		{"groovy_gradle_subprojects", "groovy", `subprojects`, true},
		{"groovy_gradle_println", "groovy", `println`, true},
		// 5. GrailsApp entry point.
		{"groovy_grailsapp_run", "groovy", `GrailsApp.run`, true},
		// Groovy also inherits all JVM reflection patterns.
		{"groovy_jvm_invoke", "groovy", `invoke`, true},
		{"groovy_jvm_forName", "groovy", `forName`, true},
		{"groovy_jvm_newInstance", "groovy", `newInstance`, true},
		{"groovy_jvm_getMethod", "groovy", `getMethod`, true},

		// Per-language gate: Groovy patterns MUST NOT fire for Java/Kotlin/Scala.
		// These names are Groovy/Grails-specific and would be user code in other JVM langs.
		{"groovy_findResource_java_neg", "java", `findResource`, false},
		{"groovy_bindData_kotlin_neg", "kotlin", `bindData`, false},
		{"groovy_mockDomains_java_neg", "java", `mockDomains`, false},
		{"groovy_thrown_java_neg", "java", `thrown`, false},
		{"groovy_doLast_java_neg", "java", `doLast`, false},
		{"groovy_doFirst_kotlin_neg", "kotlin", `doFirst`, false},
		{"groovy_mavenCentral_java_neg", "java", `mavenCentral`, false},
		{"groovy_buildscript_scala_neg", "scala", `buildscript`, false},
		{"groovy_withCriteria_java_neg", "java", `withCriteria`, false},
		{"groovy_findByEmail_java_neg", "java", `findByEmail`, false},
		{"groovy_findAllByActive_kotlin_neg", "kotlin", `findAllByActive`, false},
		// GORM names that overlap with Python/Go should NOT fire for those languages.
		{"groovy_save_python_neg", "python", `save`, false},
		{"groovy_save_go_neg", "go", `save`, false},
		{"groovy_count_python_neg", "python", `count`, false},
		{"groovy_count_go_neg", "go", `count`, false},
		{"groovy_validate_python_neg", "python", `validate`, false},
		{"groovy_validate_go_neg", "go", `validate`, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isDynamicPatternLang(tc.stub, tc.lang)
			if got != tc.want {
				t.Fatalf("isDynamicPatternLang(%q, lang=%q) = %v, want %v", tc.stub, tc.lang, got, tc.want)
			}
		})
	}
}
