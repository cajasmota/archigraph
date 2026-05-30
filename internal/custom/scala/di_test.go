package scala_test

import (
	"testing"
)

// propOf returns the value of prop k on the first SCOPE.DI entity whose name
// equals name, or "" if not found.
func diProp(ents []entitySummary, name, k string) string {
	for _, e := range ents {
		if e.Kind == "SCOPE.DI" && e.Name == name {
			return e.Props[k]
		}
	}
	return ""
}

// diNamed returns the first SCOPE.DI entity with the given subtype+name, or nil.
func diNamed(ents []entitySummary, subtype, name string) *entitySummary {
	for i := range ents {
		if ents[i].Kind == "SCOPE.DI" && ents[i].Subtype == subtype && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// DI extractor tests
// ---------------------------------------------------------------------------

func TestDIFinatraGuiceBind(t *testing.T) {
	src := `
import com.twitter.finatra.http._
import com.google.inject._
class AppModule extends TwitterModule {
  bind[UserRepository].to[UserRepositoryImpl]
  bind[OrderService].toInstance(new OrderServiceImpl())
}
`
	ents := extract(t, "custom_scala_di", fi("AppModule.scala", "scala", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.DI" && e.Subtype == "di_binding" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected di_binding from Guice bind[T].to[Impl]")
	}
}

func TestDIFinatraGuiceInject(t *testing.T) {
	src := `
import com.twitter.finatra.http._
import javax.inject.Inject
@Singleton
class UserController @Inject()(userService: UserService) extends HttpController {
  @Get("/users")
  def getUsers(request: Request): Response = ???
}
`
	ents := extract(t, "custom_scala_di", fi("UserController.scala", "scala", src))
	// Should get both a singleton scope and inject point
	foundInject := false
	foundScope := false
	for _, e := range ents {
		if e.Kind == "SCOPE.DI" && e.Subtype == "injection_point" {
			foundInject = true
		}
		if e.Kind == "SCOPE.DI" && e.Subtype == "di_scope" {
			foundScope = true
		}
	}
	if !foundInject {
		t.Error("expected injection_point from @Inject")
	}
	if !foundScope {
		t.Error("expected di_scope from @Singleton")
	}
}

func TestDIZioHttpZLayer(t *testing.T) {
	src := `
import zio._
import zio.http._
val userServiceLayer: ZLayer[Database, Nothing, UserService] =
  ZLayer.succeed(new UserServiceImpl())

val appLayer = ZLayer.make[AppEnv](
  userServiceLayer,
  databaseLayer,
  configLayer
)

val program = for {
  _ <- Server.serve(routes)
} yield ()

def run = program.provide(appLayer)
`
	ents := extract(t, "custom_scala_di", fi("App.scala", "scala", src))
	foundBinding := false
	foundInjection := false
	foundScope := false
	for _, e := range ents {
		if e.Kind == "SCOPE.DI" && e.Subtype == "di_binding" {
			foundBinding = true
		}
		if e.Kind == "SCOPE.DI" && e.Subtype == "injection_point" {
			foundInjection = true
		}
		if e.Kind == "SCOPE.DI" && e.Subtype == "di_scope" {
			foundScope = true
		}
	}
	if !foundBinding {
		t.Error("expected di_binding from ZLayer")
	}
	if !foundInjection {
		t.Error("expected injection_point from .provide()")
	}
	if !foundScope {
		t.Error("expected di_scope from typed ZLayer val")
	}
}

func TestDILagomGuice(t *testing.T) {
	src := `
import com.lightbend.lagom.scaladsl.server._
import com.google.inject.AbstractModule
class UserServiceModule extends AbstractModule {
  bind[UserService].to[UserServiceImpl]
  bind[UserRepository].toInstance(new InMemoryUserRepository())
}
@Singleton
class UserServiceImpl @Inject()(repo: UserRepository) extends UserService {
  def getUser(id: String) = repo.findById(id)
}
`
	ents := extract(t, "custom_scala_di", fi("UserServiceModule.scala", "scala", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.DI" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected DI entities from Lagom/Guice module")
	}
}

func TestDINoMatchNonDIFramework(t *testing.T) {
	// http4s doesn't have explicit DI via our extractor
	src := `
import org.http4s._
val routes = HttpRoutes.of[IO] {
  case GET -> Root / "users" => Ok("users")
}
`
	ents := extract(t, "custom_scala_di", fi("Routes.scala", "scala", src))
	if len(ents) != 0 {
		t.Errorf("expected no DI entities for http4s (not a DI-explicit framework), got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// Value-asserting deep tests (flip partial → full)
// ---------------------------------------------------------------------------

// MacWire: assert the wired binding type and member name are captured.
func TestDIMacWireBindingType(t *testing.T) {
	src := `
import com.softwaremill.macwire._
trait UserModule {
  lazy val userRepo = wire[UserRepositoryImpl]
  lazy val userService: UserService = wire[UserServiceImpl]
  val handler = wireWith(HandlerFactory.create _)
}
`
	ents := extract(t, "custom_scala_di", fi("UserModule.scala", "scala", src))

	if got := diProp(ents, "wire:userRepo→UserRepositoryImpl", "binding_type"); got != "UserRepositoryImpl" {
		t.Errorf("expected binding_type=UserRepositoryImpl for wire[UserRepositoryImpl], got %q", got)
	}
	if got := diProp(ents, "wire:userService→UserServiceImpl", "member"); got != "userService" {
		t.Errorf("expected member=userService, got %q", got)
	}
	if got := diProp(ents, "wire:userService→UserServiceImpl", "di_style"); got != "macwire" {
		t.Errorf("expected di_style=macwire, got %q", got)
	}
	if got := diProp(ents, "wireWith:handler→HandlerFactory.create", "factory"); got != "HandlerFactory.create" {
		t.Errorf("expected factory=HandlerFactory.create, got %q", got)
	}
}

// Guice Scala DSL: assert interface and impl are captured on the binding.
func TestDIGuiceScalaBindValues(t *testing.T) {
	src := `
import com.google.inject.AbstractModule
class AppModule extends AbstractModule {
  override def configure(): Unit = {
    bind[UserRepository].to[UserRepositoryImpl]
    bind[OrderService].toInstance(new OrderServiceImpl())
  }
}
`
	ents := extract(t, "custom_scala_di", fi("AppModule.scala", "scala", src))

	b := diNamed(ents, "di_binding", "bind:UserRepository→UserRepositoryImpl")
	if b == nil {
		t.Fatal("expected binding bind:UserRepository→UserRepositoryImpl")
	}
	if b.Props["interface"] != "UserRepository" || b.Props["impl"] != "UserRepositoryImpl" {
		t.Errorf("expected interface=UserRepository impl=UserRepositoryImpl, got interface=%q impl=%q",
			b.Props["interface"], b.Props["impl"])
	}
	if diProp(ents, "module:AppModule.scala", "module_base") != "AbstractModule" {
		t.Errorf("expected module_base=AbstractModule, got %q", diProp(ents, "module:AppModule.scala", "module_base"))
	}
}

// Guice Java DSL: bind(classOf[T]).to(classOf[Impl]).
func TestDIGuiceJavaBindValues(t *testing.T) {
	src := `
import com.google.inject.AbstractModule
class JavaStyleModule extends AbstractModule {
  override def configure(): Unit = {
    bind(classOf[PaymentGateway]).to(classOf[StripePaymentGateway])
  }
}
`
	ents := extract(t, "custom_scala_di", fi("JavaStyleModule.scala", "scala", src))
	b := diNamed(ents, "di_binding", "bind:PaymentGateway→StripePaymentGateway")
	if b == nil {
		t.Fatal("expected binding bind:PaymentGateway→StripePaymentGateway from Java DSL")
	}
	if b.Props["interface"] != "PaymentGateway" || b.Props["impl"] != "StripePaymentGateway" {
		t.Errorf("expected interface=PaymentGateway impl=StripePaymentGateway, got %q / %q",
			b.Props["interface"], b.Props["impl"])
	}
	if b.Props["provenance"] != "GUICE_BIND_JAVA" {
		t.Errorf("expected provenance=GUICE_BIND_JAVA, got %q", b.Props["provenance"])
	}
}

// Guice @Provides: assert provider name, return type and injected deps captured.
func TestDIGuiceProvidesDeps(t *testing.T) {
	src := `
import com.google.inject.{AbstractModule, Provides, Singleton}
class ServiceModule extends AbstractModule {
  @Provides @Singleton
  def provideUserService(repo: UserRepository, clock: Clock): UserService =
    new UserServiceImpl(repo, clock)
}
`
	ents := extract(t, "custom_scala_di", fi("ServiceModule.scala", "scala", src))
	b := diNamed(ents, "di_binding", "provides:provideUserService")
	if b == nil {
		t.Fatal("expected provides:provideUserService binding")
	}
	if b.Props["return_type"] != "UserService" {
		t.Errorf("expected return_type=UserService, got %q", b.Props["return_type"])
	}
	if b.Props["deps"] != "repo:UserRepository,clock:Clock" {
		t.Errorf("expected deps=repo:UserRepository,clock:Clock, got %q", b.Props["deps"])
	}
}

// Guice constructor injection: assert class + dep names/types captured.
func TestDIGuiceConstructorInjectionDeps(t *testing.T) {
	src := `
import com.twitter.finatra.http._
import javax.inject.Inject
class UserController @Inject()(userService: UserService, auditor: Auditor) extends HttpController
`
	ents := extract(t, "custom_scala_di", fi("UserController.scala", "scala", src))
	ip := diNamed(ents, "injection_point", "inject:UserController")
	if ip == nil {
		t.Fatal("expected injection_point inject:UserController")
	}
	if ip.Props["class"] != "UserController" {
		t.Errorf("expected class=UserController, got %q", ip.Props["class"])
	}
	if ip.Props["injection_type"] != "constructor" {
		t.Errorf("expected injection_type=constructor, got %q", ip.Props["injection_type"])
	}
	if ip.Props["deps"] != "userService:UserService,auditor:Auditor" {
		t.Errorf("expected deps=userService:UserService,auditor:Auditor, got %q", ip.Props["deps"])
	}
}

// Guice @Singleton scope: assert the scoped class name is captured.
func TestDIGuiceSingletonScope(t *testing.T) {
	src := `
import javax.inject.Singleton
@Singleton
class CacheManager
`
	ents := extract(t, "custom_scala_di", fi("CacheManager.scala", "scala", src))
	s := diNamed(ents, "di_scope", "singleton:CacheManager")
	if s == nil {
		t.Fatal("expected di_scope singleton:CacheManager")
	}
	if s.Props["scope"] != "singleton" || s.Props["scoped_class"] != "CacheManager" {
		t.Errorf("expected scope=singleton scoped_class=CacheManager, got %q / %q",
			s.Props["scope"], s.Props["scoped_class"])
	}
}

// cats-effect: assert the Resource[F, T] binding type is captured.
func TestDICatsEffectResourceBindingType(t *testing.T) {
	src := `
import cats.effect._
object Wiring {
  val userServiceR: Resource[IO, UserService] =
    Resource.make(IO(new UserServiceImpl()))(_.close())
}
`
	ents := extract(t, "custom_scala_di", fi("Wiring.scala", "scala", src))
	b := diNamed(ents, "di_binding", "resource:userServiceR→UserService")
	if b == nil {
		t.Fatal("expected resource:userServiceR→UserService binding")
	}
	if b.Props["binding_type"] != "UserService" || b.Props["member"] != "userServiceR" {
		t.Errorf("expected binding_type=UserService member=userServiceR, got %q / %q",
			b.Props["binding_type"], b.Props["member"])
	}
	if b.Props["di_style"] != "cats-effect" {
		t.Errorf("expected di_style=cats-effect, got %q", b.Props["di_style"])
	}
}

// ZIO ZLayer: assert env type, fromFunction ctor, succeed impl and provide call.
func TestDIZioZLayerValues(t *testing.T) {
	src := `
import zio._
object Layers {
  val userServiceLayer = ZLayer.fromFunction(UserServiceLive.apply _)
  val configLayer = ZLayer.succeed(AppConfig.default)
  val appLayer: ZLayer[Any, Nothing, AppEnv] = ZLayer.make[AppEnv](userServiceLayer, configLayer)
  def run = program.provide(appLayer)
}
`
	ents := extract(t, "custom_scala_di", fi("Layers.scala", "scala", src))

	if diNamed(ents, "di_binding", "zlayer:AppEnv") == nil {
		t.Error("expected env binding zlayer:AppEnv from ZLayer.make[AppEnv]")
	}
	ff := diNamed(ents, "di_binding", "zlayer:from:UserServiceLive.apply")
	if ff == nil {
		t.Fatal("expected zlayer:from:UserServiceLive.apply from ZLayer.fromFunction")
	}
	if ff.Props["constructor"] != "UserServiceLive.apply" {
		t.Errorf("expected constructor=UserServiceLive.apply, got %q", ff.Props["constructor"])
	}
	if diNamed(ents, "di_binding", "zlayer:succeed:AppConfig.default") == nil {
		t.Error("expected zlayer:succeed:AppConfig.default from ZLayer.succeed")
	}
	if diNamed(ents, "di_scope", "layer:appLayer") == nil {
		t.Error("expected di_scope layer:appLayer from typed ZLayer val")
	}
	foundProvide := false
	for _, e := range ents {
		if e.Subtype == "injection_point" && e.Props["provide_call"] == "provide" {
			foundProvide = true
		}
	}
	if !foundProvide {
		t.Error("expected injection_point with provide_call=provide from .provide(appLayer)")
	}
}

// Play Guice: @Inject() constructor in a Play controller flips on play DI.
func TestDIPlayGuiceConstructorInjection(t *testing.T) {
	src := `
import play.api.mvc._
import javax.inject.Inject
class HomeController @Inject()(cc: ControllerComponents, repo: UserRepository) extends AbstractController(cc)
`
	ents := extract(t, "custom_scala_di", fi("HomeController.scala", "scala", src))
	ip := diNamed(ents, "injection_point", "inject:HomeController")
	if ip == nil {
		t.Fatal("expected injection_point inject:HomeController for Play")
	}
	if ip.Props["framework"] != "play" {
		t.Errorf("expected framework=play, got %q", ip.Props["framework"])
	}
	if ip.Props["deps"] != "cc:ControllerComponents,repo:UserRepository" {
		t.Errorf("expected deps=cc:ControllerComponents,repo:UserRepository, got %q", ip.Props["deps"])
	}
}
