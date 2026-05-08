package resolve

import "testing"

// TestDynamicPatterns_Catalog covers every pattern in the dynamic-dispatch
// catalog (refs #44). Each row asserts that a representative call-site stub
// produced by the per-language extractors is classified as
// DispositionDynamic by isDynamicPattern.
//
// New patterns MUST land here in the same commit so the catalog stays
// regression-tested. Negative rows guard against obvious false positives —
// stubs that look reflection-adjacent but should still resolve normally.
func TestDynamicPatterns_Catalog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		stub string
		want bool
	}{
		// ---- Python -------------------------------------------------
		{"py_getattr_call", `getattr(self, name)(arg)`, true},
		{"py_getattr_dunder_name", `__getattr__`, true},
		{"py_getattr_method", `obj.__getattr__("name")`, true},
		{"py_getattribute_method", `self.__getattribute__("attr")`, true},
		{"py_setattr", `setattr(obj, "x", 1)`, true},
		{"py_globals_subscript", `globals()[name]()`, true},
		{"py_locals_subscript", `locals()[name]()`, true},
		{"py_vars_subscript", `vars()[name]()`, true},
		{"py_eval", `eval(src)`, true},
		{"py_exec", `exec(src)`, true},
		{"py_dunder_import", `__import__("os")`, true},
		{"py_importlib", `importlib.import_module("foo")`, true},
		{"py_functools_partial", `functools.partial(fn, 1)`, true},
		{"py_functools_partialmethod", `functools.partialmethod(fn)`, true},
		{"py_functools_reduce", `functools.reduce(op, xs)`, true},
		{"py_methodcaller", `operator.methodcaller("save")`, true},
		{"py_attrgetter", `operator.attrgetter("x")`, true},
		{"py_itemgetter", `operator.itemgetter(0)`, true},
		{"py_os_environ", `os.environ["HOME"]`, true},
		{"py_os_getenv", `os.getenv("HOME")`, true},
		{"py_dict_dispatch_str_key", `handlers["save"]()`, true},
		{"py_dict_dispatch_var_key", `handlers[key]()`, true},
		{"py_dotted_dispatch", `self.handlers[name](x)`, true},

		// ---- Go -----------------------------------------------------
		{"go_reflect_call", `reflect.Value.Call`, true},
		{"go_reflect_valueof", `reflect.ValueOf(x)`, true},
		{"go_method_by_name", `v.MethodByName("Foo").Call(args)`, true},
		{"go_field_by_name", `v.FieldByName("X")`, true},
		{"go_plugin_open", `plugin.Open("./mod.so")`, true},
		{"go_plugin_lookup", `p.Lookup("Sym")`, true},

		// ---- TypeScript / JavaScript -------------------------------
		{"js_reflect_apply", `Reflect.apply(fn, this, args)`, true},
		{"js_reflect_construct", `Reflect.construct(C, args)`, true},
		{"js_function_ctor", `Function("return 1")`, true},
		{"js_new_function", `new Function("return 1")`, true},
		{"js_dynamic_import", `import("./mod")`, true},
		{"js_require_dynamic", `require(modName)`, true},
		{"js_bind", `fn.bind(this)`, true},
		{"js_apply", `fn.apply(this, args)`, true},
		{"js_call", `fn.call(this, x)`, true},
		{"js_process_env", `process.env.NODE_ENV`, true},

		// ---- Ruby ---------------------------------------------------
		{"rb_send_method", `obj.send(:name)`, true},
		{"rb_bare_send", `send(:name)`, true},
		{"rb_public_send_method", `obj.public_send(:name)`, true},
		{"rb_bare_public_send", `public_send(:name)`, true},
		{"rb_dunder_send", `obj.__send__(:name)`, true},
		{"rb_method_missing_name", `method_missing`, true},
		{"rb_method_missing_call", `obj.method_missing(:foo)`, true},
		{"rb_define_method", `define_method(:foo)`, true},
		{"rb_define_method_method", `klass.define_method(:foo)`, true},
		{"rb_instance_eval", `obj.instance_eval(src)`, true},
		{"rb_class_eval", `Klass.class_eval(src)`, true},

		// ---- Java / Kotlin / JVM -----------------------------------
		{"jvm_method_invoke", `m.invoke(target, args)`, true},
		{"jvm_class_forname", `Class.forName("com.x.Y")`, true},
		{"jvm_new_instance", `Class.forName(n).newInstance()`, true},
		{"jvm_service_loader", `ServiceLoader.load(MyService.class)`, true},
		{"jvm_system_getenv", `System.getenv("HOME")`, true},

		// ---- Cross-language ----------------------------------------
		{"interpolated_template", "`prefix-${name}-suffix`", true},

		// ---- Negative cases (must NOT be dynamic) ------------------
		{"plain_kindname", `Function:Hello`, false},
		{"plain_bare_name", `Foo`, false},
		{"empty", ``, false},
		{"plain_call", `MyService.save()`, false},
		{"plain_attribute", `obj.attribute`, false},
		{"normal_function_call", `helper(x, y)`, false},
		{"structural_ref", `scope:operation:method:python:app/views.py:UserView#save`, false},
		{"ext_pkg", `ext:django`, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isDynamicPattern(tc.stub)
			if got != tc.want {
				t.Fatalf("isDynamicPattern(%q) = %v, want %v", tc.stub, got, tc.want)
			}
		})
	}
}
