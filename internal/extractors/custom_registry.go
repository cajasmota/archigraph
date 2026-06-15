// custom_registry.go wires all framework-specific custom extractors so their
// init() functions run and register against the global extractor.Registry.
//
// Add a new blank import here whenever a new internal/custom/<lang> package
// is created.
package extractors

import (
	_ "github.com/cajasmota/grafel/internal/custom/clojure"
	_ "github.com/cajasmota/grafel/internal/custom/cpp"
	_ "github.com/cajasmota/grafel/internal/custom/crystal"
	_ "github.com/cajasmota/grafel/internal/custom/csharp"
	_ "github.com/cajasmota/grafel/internal/custom/dart"
	_ "github.com/cajasmota/grafel/internal/custom/elixir"
	_ "github.com/cajasmota/grafel/internal/custom/erlang"
	_ "github.com/cajasmota/grafel/internal/custom/fsharp"
	_ "github.com/cajasmota/grafel/internal/custom/golang"
	_ "github.com/cajasmota/grafel/internal/custom/groovy"
	_ "github.com/cajasmota/grafel/internal/custom/java"
	_ "github.com/cajasmota/grafel/internal/custom/javascript"
	_ "github.com/cajasmota/grafel/internal/custom/kotlin"
	_ "github.com/cajasmota/grafel/internal/custom/lua"
	_ "github.com/cajasmota/grafel/internal/custom/nim"
	_ "github.com/cajasmota/grafel/internal/custom/php"
	_ "github.com/cajasmota/grafel/internal/custom/python"
	_ "github.com/cajasmota/grafel/internal/custom/ruby"
	_ "github.com/cajasmota/grafel/internal/custom/rust"
	_ "github.com/cajasmota/grafel/internal/custom/scala"
	_ "github.com/cajasmota/grafel/internal/custom/swift"
)
