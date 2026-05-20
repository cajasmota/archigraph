package resolve

import "testing"

// TestDynamicPatterns_JVM covers the jvmDynamicPatterns catalog for
// Java, Kotlin, Scala, and JVM language tags.
func TestDynamicPatterns_JVM(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lang string
		stub string
		want bool
	}{
		{"jvm_method_invoke", "java", `m.Method.invoke(target, args)`, true},
		{"jvm_method_invoke_qualified", "java", `Method.invoke(target, args)`, true},
		{"jvm_constructor_invoke", "java", `Constructor.invoke(args)`, true},
		{"jvm_class_forname", "java", `Class.forName("com.x.Y")`, true},
		{"jvm_new_instance", "java", `Class.forName(n).newInstance()`, true},
		{"jvm_class_class_newinstance", "kotlin", `MyType.class.newInstance()`, true},
		{"jvm_service_loader", "java", `ServiceLoader.load(MyService.class)`, true},
		{"jvm_system_getenv", "java", `System.getenv("HOME")`, true},
		// Bare-identifier forms (issue #72).
		{"jvm_bare_forName", "java", `forName`, true},
		{"jvm_bare_invoke_java", "java", `invoke`, true},
		{"jvm_bare_invoke_kotlin", "kotlin", `invoke`, true},
		{"jvm_bare_newInstance", "java", `newInstance`, true},
		{"jvm_bare_getClass", "java", `getClass`, true},
		{"jvm_bare_getMethod", "java", `getMethod`, true},
		{"jvm_bare_getMethods", "java", `getMethods`, true},
		{"jvm_bare_getDeclaredMethod", "java", `getDeclaredMethod`, true},
		{"jvm_bare_getDeclaredMethods", "java", `getDeclaredMethods`, true},
		{"jvm_bare_getField", "java", `getField`, true},
		{"jvm_bare_getFields", "java", `getFields`, true},
		{"jvm_bare_getDeclaredField", "java", `getDeclaredField`, true},
		{"jvm_bare_getDeclaredFields", "java", `getDeclaredFields`, true},
		{"jvm_bare_getConstructor", "java", `getConstructor`, true},
		{"jvm_bare_getConstructors", "java", `getConstructors`, true},
		{"jvm_bare_getDeclaredConstructor", "java", `getDeclaredConstructor`, true},
		{"jvm_bare_getDeclaredConstructors", "java", `getDeclaredConstructors`, true},
		// Spring MVC ResponseEntity fluent builder methods (issue #44).
		{"spring_kotlin_notFound", "kotlin", `notFound`, true},
		{"spring_kotlin_noContent", "kotlin", `noContent`, true},
		{"spring_kotlin_badRequest", "kotlin", `badRequest`, true},
		{"spring_kotlin_accepted", "kotlin", `accepted`, true},
		{"spring_kotlin_created", "kotlin", `created`, true},
		{"spring_kotlin_ok", "kotlin", `ok`, true},
		{"spring_kotlin_build", "kotlin", `build`, true},
		{"spring_kotlin_body", "kotlin", `body`, true},
		{"spring_kotlin_unprocessableEntity", "kotlin", `unprocessableEntity`, true},
		{"spring_kotlin_internalServerError", "kotlin", `internalServerError`, true},
		{"spring_java_notFound", "java", `notFound`, true},
		{"spring_java_build", "java", `build`, true},
		{"spring_java_ok", "java", `ok`, true},
		{"spring_scala_noContent", "scala", `noContent`, true},
		// Cross-language gate: Spring builder names MUST NOT fire for non-JVM.
		{"spring_python_build_neg", "python", `build`, false},
		{"spring_js_ok_neg", "javascript", `ok`, false},
		{"spring_ruby_notFound_neg", "ruby", `notFound`, false},
		{"spring_rust_build_neg", "rust", `build`, false},
		// Scala stdlib companion-object + collection methods (issue #44).
		{"scala_future_successful", "scala", `Future.successful`, true},
		{"scala_future_failed", "scala", `Future.failed`, true},
		{"scala_future_apply", "scala", `Future.apply`, true},
		{"scala_future_sequence", "scala", `Future.sequence`, true},
		{"scala_try_apply", "scala", `Try.apply`, true},
		{"scala_success_apply", "scala", `Success.apply`, true},
		{"scala_failure_apply", "scala", `Failure.apply`, true},
		{"scala_list_map", "scala", `List.map`, true},
		{"scala_list_flatmap", "scala", `List.flatMap`, true},
		{"scala_list_filter", "scala", `List.filter`, true},
		{"scala_list_empty", "scala", `List.empty`, true},
		{"scala_list_apply", "scala", `List.apply`, true},
		{"scala_map_get", "scala", `Map.get`, true},
		{"scala_map_empty", "scala", `Map.empty`, true},
		{"scala_seq_apply", "scala", `Seq.apply`, true},
		{"scala_vector_apply", "scala", `Vector.apply`, true},
		{"scala_set_apply", "scala", `Set.apply`, true},
		{"scala_option_apply", "scala", `Option.apply`, true},
		{"scala_some_apply", "scala", `Some.apply`, true},
		// JVM gate: Scala stdlib qualified names also fire for Kotlin/Java.
		{"scala_future_successful_kotlin", "kotlin", `Future.successful`, true},
		{"scala_future_successful_java", "java", `Future.successful`, true},
		{"scala_list_map_kotlin", "kotlin", `List.map`, true},
		// Cross-language gate: Scala stdlib MUST NOT fire for non-JVM.
		{"scala_future_successful_python_neg", "python", `Future.successful`, false},
		{"scala_future_successful_go_neg", "go", `Future.successful`, false},
		{"scala_future_successful_js_neg", "javascript", `Future.successful`, false},
		{"scala_future_successful_ruby_neg", "ruby", `Future.successful`, false},
		{"scala_list_map_python_neg", "python", `List.map`, false},
		{"scala_map_get_python_neg", "python", `Map.get`, false},
		// factory.newInstance() — domain factory, not reflective.
		{"neg_factory_newinstance", "java", `factory.newInstance()`, false},
		// cli.invoke(...) — user-defined invoke, not Method.invoke.
		{"neg_cli_invoke", "java", `cli.invoke(cmd, args)`, false},
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
