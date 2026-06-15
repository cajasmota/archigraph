// Package scala — DI extractor for Scala DI mechanisms.
//
// Covers (partial → full where value-asserted):
//
//	Record                   Capability                   Status
//	───────────────────────────────────────────────────────────
//	framework.finatra        DI/di_binding_extraction     full
//	framework.finatra        DI/di_injection_point        full
//	framework.finatra        DI/di_scope_resolution       full
//	framework.lagom          DI/di_binding_extraction     full
//	framework.lagom          DI/di_injection_point        full
//	framework.lagom          DI/di_scope_resolution       full
//	framework.zio-http       DI/di_binding_extraction     full
//	framework.zio-http       DI/di_injection_point        full
//	framework.zio-http       DI/di_scope_resolution       full
//	framework.play           DI/di_binding_extraction     full
//	framework.play           DI/di_injection_point        full
//	framework.play           DI/di_scope_resolution       full
//
// DI mechanisms (detected independently of the HTTP framework, since DI wiring
// frequently lives in module/bootstrap files that don't import the framework):
//
//	MacWire (com.softwaremill.macwire):
//	  - val svc = wire[UserServiceImpl]            — macro constructor wiring
//	  - lazy val repo = wire[UserRepository]       — lazy module member wiring
//	  - val h = wireWith(Factory.create _)         — factory-function wiring
//	  binding type captured from wire[T] / wireWith.
//
//	Guice (Finatra / Lagom / Play / plain):
//	  - Scala DSL : bind[Service].to[ServiceImpl] / bind[T].toInstance(...)
//	  - Java DSL  : bind(classOf[Service]).to(classOf[ServiceImpl])
//	  - @Provides [@Singleton] def f(dep: A, dep2: B): T — provider + deps
//	  - class C @Inject()(a: A, b: B) — constructor injection + dep names/types
//	  - @Singleton class C — singleton scope
//	  - extends AbstractModule / TwitterModule — module declaration
//
//	cats-effect (org.typelevel.cats.effect):
//	  - val r: Resource[IO, UserService] = Resource.make(acquire)(release)
//	  - Resource.eval / Resource.pure / Resource.fromAutoCloseable acquisition
//	  binding type captured from the Resource[F, T] type argument.
//
//	ZIO (dev.zio):
//	  - ZLayer.make[Env](...) / ZLayer.succeed(impl) / ZLayer.fromFunction(ctor)
//	  - val l: ZLayer[R, E, A] = ...                — typed layer val
//	  - program.provide(appLayer) / provideLayer / provideSomeLayer — injection
//
// AOP/Transactions are framework-level and out of scope for this extractor.
//
// Honest limit: regex-based, file-local. Cross-file binding resolution (which
// concrete module a bind[] lands in, which file an injected dep is defined in)
// is NOT performed — each binding/injection/scope record is emitted with the
// specific local names it asserts (interface, impl, dep names+types, env type).
package scala

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_scala_di", &scalaDIExtractor{})
}

type scalaDIExtractor struct{}

func (e *scalaDIExtractor) Language() string { return "custom_scala_di" }

// ---------------------------------------------------------------------------
// MacWire regexes
// ---------------------------------------------------------------------------

var (
	// diMacWire matches `[lazy] val name = wire[Type]` — captures member name
	// (group 1) and the wired type (group 2).
	diMacWire = regexp.MustCompile(
		`\b(?:lazy\s+)?val\s+(\w+)\s*(?::[^=]+)?=\s*wire\s*\[\s*([A-Z][\w.]*)\s*\]`)

	// diMacWireWith matches `[lazy] val name = wireWith(factory)` — captures
	// member name (group 1) and the factory reference (group 2).
	diMacWireWith = regexp.MustCompile(
		`\b(?:lazy\s+)?val\s+(\w+)\s*(?::[^=]+)?=\s*wireWith\s*\(\s*([\w.]+)`)
)

// ---------------------------------------------------------------------------
// Guice regexes
// ---------------------------------------------------------------------------

var (
	// diGuiceInjectCtor matches a constructor-injection class declaration and
	// captures the class name (group 1) and the raw parameter list (group 2),
	// e.g. `class UserController @Inject()(svc: UserService, repo: Repo)`.
	diGuiceInjectCtor = regexp.MustCompile(
		`class\s+(\w+)\s+@Inject\s*\(\s*\)\s*\(([^)]*)\)`)

	// diGuiceInjectBare matches a bare @Inject (field/method injection or a
	// constructor whose param list we did not capture).
	diGuiceInjectBare = regexp.MustCompile(`@Inject\s*(?:\(\s*\))?`)

	// diGuiceSingleton matches @Singleton class annotation.
	diGuiceSingleton = regexp.MustCompile(
		`@Singleton\s+(?:final\s+)?(?:case\s+)?class\s+(\w+)`)

	// diGuiceBindScala matches the Scala DSL bind[Type].to[Impl] /
	// .toInstance / .toSelf / .in[Scope]. Group1 = interface, group2 = impl.
	diGuiceBindScala = regexp.MustCompile(
		`\bbind\s*\[\s*([A-Z][\w.]*)\s*\]\s*\.\s*(?:to\s*\[\s*([A-Z][\w.]*)\s*\]|toInstance|toSelf|in\s*\[)`)

	// diGuiceBindJava matches the Java DSL bind(classOf[Type]).to(classOf[Impl]).
	// Group1 = interface, group2 = impl (impl optional for toInstance).
	diGuiceBindJava = regexp.MustCompile(
		`\bbind\s*\(\s*classOf\s*\[\s*([A-Z][\w.]*)\s*\]\s*\)\s*\.\s*to\s*\(\s*classOf\s*\[\s*([A-Z][\w.]*)\s*\]`)

	// diGuiceModule matches the module base classes.
	diGuiceModule = regexp.MustCompile(
		`\bextends\s+(TwitterModule|AbstractModule|LagomServicePortBindings|ServiceLocatorModule)\b`)

	// diGuiceProvides matches @Provides [@Singleton] def name(params): RetT.
	// Group1 = method name, group2 = param list, group3 = return type (opt).
	diGuiceProvides = regexp.MustCompile(
		`@Provides\s+(?:@Singleton\s+)?def\s+(\w+)\s*\(([^)]*)\)\s*(?::\s*([A-Z][\w.\[\]]*))?`)
)

// ---------------------------------------------------------------------------
// cats-effect regexes
// ---------------------------------------------------------------------------

var (
	// diCatsResourceTyped matches `val name: Resource[F, Type] = ...` — captures
	// the val name (group 1) and the produced resource type (group 2).
	diCatsResourceTyped = regexp.MustCompile(
		`\bval\s+(\w+)\s*:\s*Resource\s*\[\s*[\w.]+\s*,\s*([A-Z][\w.]*)\s*\]`)

	// diCatsResourceMake matches Resource.make / Resource.eval / Resource.pure /
	// Resource.fromAutoCloseable acquisition calls.
	diCatsResourceMake = regexp.MustCompile(
		`\bResource\s*\.\s*(make|makeCase|eval|pure|fromAutoCloseable|liftK)\b`)
)

// ---------------------------------------------------------------------------
// ZIO ZLayer regexes
// ---------------------------------------------------------------------------

var (
	// diZLayerMake matches ZLayer.make[Env] / makeSome / makeWith.
	diZLayerMake = regexp.MustCompile(
		`\bZLayer\s*\.\s*(?:make|makeSome|makeWith)\s*\[\s*([A-Z][\w\s.,]*?)\s*\]`)

	// diZLayerSucceed matches ZLayer.succeed(impl) — captures the supplied
	// implementation reference (group 1) where it is a simple identifier/ctor.
	diZLayerSucceed = regexp.MustCompile(
		`\bZLayer\s*\.\s*succeed\s*\(\s*(?:new\s+)?([A-Za-z_][\w.]*)?`)

	// diZLayerFromFunction matches ZLayer.fromFunction(ctor) — captures the
	// constructor/function reference (group 1).
	diZLayerFromFunction = regexp.MustCompile(
		`\bZLayer\s*\.\s*(?:fromFunction|fromZIO|fromManaged|scoped|apply)\s*\(\s*(?:new\s+)?([A-Za-z_][\w.]*)?`)

	// diZLayerVal matches `val name: ZLayer[R, E, A] = ...` typed layer val.
	diZLayerVal = regexp.MustCompile(
		`\bval\s+(\w+)\s*:\s*ZLayer\s*\[`)

	// diZLayerProvide matches .provide / .provideLayer / .provideSomeLayer.
	diZLayerProvide = regexp.MustCompile(
		`\.\s*(provide|provideLayer|provideSomeLayer|provideEnvironment|inject)\s*\(`)
)

// ---------------------------------------------------------------------------
// detection
// ---------------------------------------------------------------------------

type diStyles struct {
	macwire bool
	guice   bool
	cats    bool
	zio     bool
}

// detectDIStyles reports which DI mechanisms are present in src. It is
// deliberately independent of the HTTP-framework detector: DI wiring routinely
// lives in module/bootstrap files that do not import the web framework.
func detectDIStyles(src string, fw string) diStyles {
	var s diStyles
	if strings.Contains(src, "macwire") || strings.Contains(src, "wire[") ||
		strings.Contains(src, "wire [") || strings.Contains(src, "wireWith") {
		s.macwire = true
	}
	if fw == "finatra" || fw == "lagom" || fw == "play" ||
		strings.Contains(src, "com.google.inject") || strings.Contains(src, "javax.inject") ||
		strings.Contains(src, "jakarta.inject") || strings.Contains(src, "AbstractModule") ||
		strings.Contains(src, "TwitterModule") || strings.Contains(src, "@Provides") ||
		(strings.Contains(src, "@Inject") && !s.macwire) {
		s.guice = true
	}
	if strings.Contains(src, "cats.effect") || strings.Contains(src, "Resource[") ||
		strings.Contains(src, "Resource.make") || strings.Contains(src, "Resource.eval") {
		s.cats = true
	}
	if fw == "zio-http" || strings.Contains(src, "dev.zio") || strings.Contains(src, "ZLayer") {
		s.zio = true
	}
	return s
}

// splitInjectedDeps parses a Scala parameter list like
// "svc: UserService, repo: Repo = default" into "name:Type" tokens, dropping
// default-value RHS and modifiers. Returns up to a stable comma-joined string.
func splitInjectedDeps(params string) []string {
	params = strings.TrimSpace(params)
	if params == "" {
		return nil
	}
	var out []string
	for _, raw := range strings.Split(params, ",") {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		// drop default value RHS
		if i := strings.Index(p, "="); i >= 0 {
			p = strings.TrimSpace(p[:i])
		}
		// "name: Type" → name + type
		if i := strings.Index(p, ":"); i >= 0 {
			name := strings.TrimSpace(p[:i])
			typ := strings.TrimSpace(p[i+1:])
			// strip leading modifiers (val/var/implicit) from name token
			nf := strings.Fields(name)
			if len(nf) > 0 {
				name = nf[len(nf)-1]
			}
			if name != "" && typ != "" {
				out = append(out, name+":"+typ)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *scalaDIExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/scala")
	_, span := tracer.Start(ctx, "indexer.scala_di.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("extractor", "di"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "scala" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	fw := detectScalaFramework(src)
	st := detectDIStyles(src, fw)

	if !st.macwire && !st.guice && !st.cats && !st.zio {
		return nil, nil
	}

	// -----------------------------------------------------------------------
	// MacWire — di_binding_extraction (binding type captured)
	// -----------------------------------------------------------------------
	if st.macwire {
		for _, m := range diMacWire.FindAllStringSubmatchIndex(src, -1) {
			member := src[m[2]:m[3]]
			typ := src[m[4]:m[5]]
			ent := makeEntity("wire:"+member+"→"+typ, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "macwire", "member", member, "binding_type", typ, "provenance", "MACWIRE_WIRE")
			add(ent)
		}
		for _, m := range diMacWireWith.FindAllStringSubmatchIndex(src, -1) {
			member := src[m[2]:m[3]]
			factory := src[m[4]:m[5]]
			ent := makeEntity("wireWith:"+member+"→"+factory, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "macwire", "member", member, "factory", factory, "provenance", "MACWIRE_WIREWITH")
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Guice — bindings, providers, injection points, scopes, modules
	// -----------------------------------------------------------------------
	if st.guice {
		// Scala DSL bindings: bind[T].to[Impl]
		for _, m := range diGuiceBindScala.FindAllStringSubmatchIndex(src, -1) {
			iface := src[m[2]:m[3]]
			impl := ""
			if m[4] >= 0 {
				impl = src[m[4]:m[5]]
			}
			name := "bind:" + iface
			if impl != "" {
				name += "→" + impl
			}
			ent := makeEntity(name, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "interface", iface, "impl", impl, "provenance", "GUICE_BIND_SCALA")
			add(ent)
		}
		// Java DSL bindings: bind(classOf[T]).to(classOf[Impl])
		for _, m := range diGuiceBindJava.FindAllStringSubmatchIndex(src, -1) {
			iface := src[m[2]:m[3]]
			impl := src[m[4]:m[5]]
			ent := makeEntity("bind:"+iface+"→"+impl, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "interface", iface, "impl", impl, "provenance", "GUICE_BIND_JAVA")
			add(ent)
		}
		// @Provides methods — provider binding + injected deps
		for _, m := range diGuiceProvides.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			params := ""
			if m[4] >= 0 {
				params = src[m[4]:m[5]]
			}
			retType := ""
			if m[6] >= 0 {
				retType = src[m[6]:m[7]]
			}
			deps := splitInjectedDeps(params)
			ent := makeEntity("provides:"+name, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "provides_method", name,
				"return_type", retType, "deps", strings.Join(deps, ","), "provenance", "GUICE_PROVIDES")
			add(ent)
		}

		// Constructor injection: class C @Inject()(deps) — captures dep names+types
		ctorInjectSeen := make(map[int]bool)
		for _, m := range diGuiceInjectCtor.FindAllStringSubmatchIndex(src, -1) {
			cls := src[m[2]:m[3]]
			params := src[m[4]:m[5]]
			ctorInjectSeen[m[0]] = true
			deps := splitInjectedDeps(params)
			ent := makeEntity("inject:"+cls, "SCOPE.DI", "injection_point", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "class", cls,
				"injection_type", "constructor", "deps", strings.Join(deps, ","), "provenance", "GUICE_INJECT_CTOR")
			add(ent)
		}
		// Bare @Inject (field/method) not already accounted for by a ctor match.
		for _, m := range diGuiceInjectBare.FindAllStringSubmatchIndex(src, -1) {
			// Skip if this @Inject is the start of a captured constructor.
			skip := false
			for pos := range ctorInjectSeen {
				if m[0] >= pos && m[0] <= pos+8 {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			ent := makeEntity("inject:"+fileBaseName(file.Path)+":"+lineStr(src, m[0]), "SCOPE.DI", "injection_point", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "injection_type", "field_or_method", "provenance", "GUICE_INJECT_BARE")
			add(ent)
		}

		// @Singleton scope
		for _, m := range diGuiceSingleton.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity("singleton:"+name, "SCOPE.DI", "di_scope", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "scope", "singleton", "scoped_class", name, "provenance", "GUICE_SINGLETON")
			add(ent)
		}
		// Module declarations
		for _, m := range diGuiceModule.FindAllStringSubmatchIndex(src, -1) {
			base := src[m[2]:m[3]]
			ent := makeEntity("module:"+fileBaseName(file.Path), "SCOPE.DI", "di_module", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "guice", "module_base", base, "provenance", "GUICE_MODULE")
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// cats-effect — Resource bindings
	// -----------------------------------------------------------------------
	if st.cats {
		for _, m := range diCatsResourceTyped.FindAllStringSubmatchIndex(src, -1) {
			member := src[m[2]:m[3]]
			typ := src[m[4]:m[5]]
			ent := makeEntity("resource:"+member+"→"+typ, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "cats-effect", "member", member, "binding_type", typ, "provenance", "CATS_RESOURCE_TYPED")
			add(ent)
		}
		for _, m := range diCatsResourceMake.FindAllStringSubmatchIndex(src, -1) {
			acq := src[m[2]:m[3]]
			ent := makeEntity("resource:make:"+fileBaseName(file.Path)+":"+lineStr(src, m[0]), "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", fw, "di_style", "cats-effect", "acquire_kind", acq, "provenance", "CATS_RESOURCE_MAKE")
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// ZIO ZLayer — bindings, injection points, typed-layer scopes
	// -----------------------------------------------------------------------
	if st.zio {
		// di_binding: ZLayer.make[Env]
		for _, m := range diZLayerMake.FindAllStringSubmatchIndex(src, -1) {
			envType := strings.TrimSpace(src[m[2]:m[3]])
			ent := makeEntity("zlayer:"+envType, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "zio-http", "di_style", "zio", "env_type", envType, "provenance", "ZIO_ZLAYER_MAKE")
			add(ent)
		}
		// di_binding: ZLayer.succeed(impl) — capture impl reference
		for _, m := range diZLayerSucceed.FindAllStringSubmatchIndex(src, -1) {
			impl := ""
			if m[2] >= 0 {
				impl = src[m[2]:m[3]]
			}
			name := "zlayer:succeed:" + fileBaseName(file.Path) + ":" + lineStr(src, m[0])
			if impl != "" {
				name = "zlayer:succeed:" + impl
			}
			ent := makeEntity(name, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "zio-http", "di_style", "zio", "impl", impl, "provenance", "ZIO_ZLAYER_SUCCEED")
			add(ent)
		}
		// di_binding: ZLayer.fromFunction(ctor) etc.
		for _, m := range diZLayerFromFunction.FindAllStringSubmatchIndex(src, -1) {
			ctor := ""
			if m[2] >= 0 {
				ctor = src[m[2]:m[3]]
			}
			name := "zlayer:from:" + fileBaseName(file.Path) + ":" + lineStr(src, m[0])
			if ctor != "" {
				name = "zlayer:from:" + ctor
			}
			ent := makeEntity(name, "SCOPE.DI", "di_binding", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "zio-http", "di_style", "zio", "constructor", ctor, "provenance", "ZIO_ZLAYER_FROMFUNCTION")
			add(ent)
		}
		// di_scope: typed ZLayer val
		for _, m := range diZLayerVal.FindAllStringSubmatchIndex(src, -1) {
			name := src[m[2]:m[3]]
			ent := makeEntity("layer:"+name, "SCOPE.DI", "di_scope", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "zio-http", "di_style", "zio", "scope", "zlayer", "layer_val", name, "provenance", "ZIO_ZLAYER_VAL")
			add(ent)
		}
		// injection_point: provide / provideLayer / provideSomeLayer / inject
		for _, m := range diZLayerProvide.FindAllStringSubmatchIndex(src, -1) {
			call := src[m[2]:m[3]]
			ent := makeEntity("provide:"+fileBaseName(file.Path)+":"+lineStr(src, m[0]), "SCOPE.DI", "injection_point", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "zio-http", "di_style", "zio", "provide_call", call, "provenance", "ZIO_PROVIDE_LAYER")
			add(ent)
		}
	}

	return entities, nil
}
