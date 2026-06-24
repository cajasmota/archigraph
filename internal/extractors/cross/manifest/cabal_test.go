package manifest

import "testing"

// realCabal is a representative *.cabal manifest exercising a library stanza
// (runtime build-depends spanning multiple comma-led continuation lines), a
// test-suite stanza (dev deps), the `base` GHC floor, a bare dep, and version
// constraints.
const realCabal = `cabal-version:      2.4
name:               mylib
version:            0.1.0.0

library
    exposed-modules:  MyLib
    build-depends:    base >=4.14 && <5
                    , text
                    , aeson >= 2.0
    hs-source-dirs:   src
    default-language: Haskell2010

test-suite spec
    type:             exitcode-stdio-1.0
    main-is:          Spec.hs
    build-depends:    base, hspec >= 2.7, mylib
    hs-source-dirs:   test
`

func TestCabal_LibraryAndTestDeps(t *testing.T) {
	deps := depEntities(runExtract(t, "mylib.cabal", realCabal))
	// runtime: base, text, aeson (+ base seen first so it stays runtime) ;
	// dev-only new names: hspec, mylib. base/aeson/text first-declaration-wins.
	for _, d := range deps {
		if d.Properties["package_manager"] != "cabal" {
			t.Errorf("%s: package_manager=%q want cabal", d.Name, d.Properties["package_manager"])
		}
	}

	if d := depByName(deps, "base"); d == nil {
		t.Error("expected runtime dep 'base' (GHC floor is a real edge)")
	} else if d.Properties["version"] != ">=4.14 && <5" {
		t.Errorf("base version=%q want '>=4.14 && <5'", d.Properties["version"])
	} else if d.Properties["is_dev"] != "false" {
		t.Errorf("base should be is_dev=false (first declared in library)")
	}

	if d := depByName(deps, "aeson"); d == nil || d.Properties["version"] != ">= 2.0" {
		t.Errorf("aeson version=%v want '>= 2.0'", d)
	}
	if d := depByName(deps, "text"); d == nil {
		t.Error("expected dep 'text'")
	} else if d.Properties["version"] != "" {
		t.Errorf("text version=%q want empty (bare)", d.Properties["version"])
	}

	// hspec is declared only in the test-suite stanza → is_dev=true.
	if d := depByName(deps, "hspec"); d == nil {
		t.Error("expected dev dep 'hspec' from test-suite stanza")
	} else if d.Properties["is_dev"] != "true" {
		t.Errorf("hspec should be is_dev=true (test-suite stanza); got %q", d.Properties["is_dev"])
	}
	if d := depByName(deps, "mylib"); d == nil || d.Properties["is_dev"] != "true" {
		t.Errorf("mylib should be is_dev=true (test-suite stanza); got %v", d)
	}
}

// realPackageYaml is a representative hpack package.yaml with a top-level
// runtime dependencies list and a per-test dependencies block.
const realPackageYaml = `name:    mylib
version: 0.1.0.0

dependencies:
  - base >= 4.14 && < 5
  - text
  - aeson

library:
  source-dirs: src

tests:
  mylib-test:
    main: Spec.hs
    source-dirs: test
    dependencies:
      - hspec
`

func TestPackageYaml_RuntimeAndTestDeps(t *testing.T) {
	deps := depEntities(runExtract(t, "package.yaml", realPackageYaml))
	for _, d := range deps {
		if d.Properties["package_manager"] != "hpack" {
			t.Errorf("%s: package_manager=%q want hpack", d.Name, d.Properties["package_manager"])
		}
	}
	if d := depByName(deps, "base"); d == nil {
		t.Error("expected runtime dep 'base'")
	} else if d.Properties["version"] != ">= 4.14 && < 5" {
		t.Errorf("base version=%q want '>= 4.14 && < 5'", d.Properties["version"])
	}
	if d := depByName(deps, "aeson"); d == nil {
		t.Error("expected runtime dep 'aeson'")
	}
	if d := depByName(deps, "hspec"); d == nil {
		t.Error("expected dev dep 'hspec' from tests: section")
	} else if d.Properties["is_dev"] != "true" {
		t.Errorf("hspec should be is_dev=true (tests: section); got %q", d.Properties["is_dev"])
	}
}

// realStackYaml is a representative stack.yaml with a versioned extra-dep, a
// second versioned extra-dep, and a git source-pinned extra-dep.
const realStackYaml = `resolver: lts-21.0

packages:
  - .

extra-deps:
  - acme-missiles-0.3
  - text-2.0.1
  - github: foo/bar
    commit: abc123
`

func TestStackYaml_ExtraDeps(t *testing.T) {
	deps := depEntities(runExtract(t, "stack.yaml", realStackYaml))
	for _, d := range deps {
		if d.Properties["package_manager"] != "stack" {
			t.Errorf("%s: package_manager=%q want stack", d.Name, d.Properties["package_manager"])
		}
	}
	if d := depByName(deps, "acme-missiles"); d == nil {
		t.Error("expected extra-dep 'acme-missiles'")
	} else if d.Properties["version"] != "0.3" {
		t.Errorf("acme-missiles version=%q want '0.3'", d.Properties["version"])
	}
	if d := depByName(deps, "text"); d == nil || d.Properties["version"] != "2.0.1" {
		t.Errorf("text version=%v want '2.0.1'", d)
	}
	// git source-pinned repo name recorded with empty version.
	if d := depByName(deps, "bar"); d == nil {
		t.Error("expected git source-pinned extra-dep 'bar'")
	}
	// The resolver/snapshot itself must NOT be enumerated as a dependency.
	if d := depByName(deps, "lts-21.0"); d != nil {
		t.Error("resolver/snapshot must not be emitted as a dependency")
	}
}

// TestCabal_PackageYaml_NotManifestNeg guards that a plain non-Haskell YAML
// named neither stack.yaml/package.yaml nor *.cabal is not parsed here.
func TestCabal_NonManifestExtensionIgnored(t *testing.T) {
	if IsManifest("src/Main.hs") {
		t.Error("a .hs source file must not be treated as a manifest")
	}
	if !IsManifest("mylib.cabal") {
		t.Error("*.cabal must be recognised as a manifest")
	}
	if !IsManifest("stack.yaml") || !IsManifest("package.yaml") {
		t.Error("stack.yaml / package.yaml must be recognised as manifests")
	}
}
