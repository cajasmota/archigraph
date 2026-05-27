// Per-language smoke tests for the T3 (#2763) Phase 0 sniffers. Each test
// asserts (a) the literal-binding shape, (b) the env-fallback shape where
// applicable, and (c) one cross-file import binding, so a regression in any
// of the three sniffer pillars is caught at unit-test time.
//
// Languages without a meaningful env-fallback idiom (Elm, Haskell, Solidity,
// ReasonML, Zig) skip the env assertion — see each sniffer's docstring for
// the deliberate omission.
package substrate

import "testing"

// byIdent collects Bindings into a name-keyed map for assertion ergonomics.
func byIdent(bs []Binding) map[string]Binding {
	m := map[string]Binding{}
	for _, b := range bs {
		m[b.Ident] = b
	}
	return m
}

func TestDartSniffer(t *testing.T) {
	const src = `import 'package:foo/bar.dart';
import 'package:other/thing.dart' as thing;
const API = "https://api.example.com";
final NAME = 'literal';
final PORT = Platform.environment["PORT"] ?? "8080";
const DB = String.fromEnvironment("DB_URL", defaultValue: "sqlite::memory:");
`
	b := byIdent(sniffDart(src))
	if b["API"].Value != "https://api.example.com" || b["API"].Provenance != ProvenanceLiteral {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["DB"].Value != "sqlite::memory:" || b["DB"].EnvVar != "DB_URL" {
		t.Errorf("DB: %+v", b["DB"])
	}
	if b["bar"].ImportSource != "package:foo/bar.dart" {
		t.Errorf("bar import: %+v", b["bar"])
	}
	if b["thing"].ImportSource != "package:other/thing.dart" {
		t.Errorf("thing aliased import: %+v", b["thing"])
	}
}

func TestGroovySniffer(t *testing.T) {
	const src = `import com.example.Util
import static com.example.Helper.HELP
final String API = "https://api.example.com"
def NAME = 'literal'
static final String PORT = System.getenv("PORT") ?: "8080"
`
	b := byIdent(sniffGroovy(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["Util"].ImportSource != "com.example" {
		t.Errorf("Util: %+v", b["Util"])
	}
	if b["HELP"].ImportSource != "com.example.Helper" {
		t.Errorf("HELP static import: %+v", b["HELP"])
	}
}

func TestLuaSniffer(t *testing.T) {
	const src = `local M = require("other.mod")
local cjson = require "cjson"
API = "https://api.example.com"
local NAME = 'literal'
local PORT = os.getenv("PORT") or "8080"
`
	b := byIdent(sniffLua(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["M"].ImportSource != "other.mod" {
		t.Errorf("M require: %+v", b["M"])
	}
	if b["cjson"].ImportSource != "cjson" {
		t.Errorf("cjson require: %+v", b["cjson"])
	}
}

func TestSwiftSniffer(t *testing.T) {
	const src = `import Foundation
import struct Foundation.Date
let API = "https://api.example.com"
static let NAME: String = "literal"
let PORT = ProcessInfo.processInfo.environment["PORT"] ?? "8080"
`
	b := byIdent(sniffSwift(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["Foundation"].ImportSource != "Foundation" {
		t.Errorf("Foundation: %+v", b["Foundation"])
	}
	if b["Date"].ImportSource != "Foundation.Date" {
		t.Errorf("Date import: %+v", b["Date"])
	}
}

func TestClojureSniffer(t *testing.T) {
	const src = `(ns app.core
  (:require [clojure.string :as str]
            [other.ns]))

(def API "https://api.example.com")
(def PORT (or (System/getenv "PORT") "8080"))
`
	b := byIdent(sniffClojure(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["str"].ImportSource != "clojure.string" {
		t.Errorf("str alias: %+v", b["str"])
	}
}

func TestCrystalSniffer(t *testing.T) {
	const src = `require "http/server"
API = "https://api.example.com"
PORT = ENV["PORT"]? || "8080"
DB = ENV.fetch("DB_URL", "sqlite::memory:")
`
	b := byIdent(sniffCrystal(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["DB"].Value != "sqlite::memory:" || b["DB"].EnvVar != "DB_URL" {
		t.Errorf("DB: %+v", b["DB"])
	}
	if b["http/server"].ImportSource != "http/server" {
		t.Errorf("require: %+v", b["http/server"])
	}
}

func TestElmSniffer(t *testing.T) {
	const src = `import Html exposing (..)
import Json.Decode as Decode
api = "https://api.example.com"
name = "literal"
`
	b := byIdent(sniffElm(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["Html"].ImportSource != "Html" {
		t.Errorf("Html import: %+v", b["Html"])
	}
	if b["Decode"].ImportSource != "Json.Decode" {
		t.Errorf("Decode aliased: %+v", b["Decode"])
	}
}

func TestErlangSniffer(t *testing.T) {
	const src = `-module(app).
-import(lists, [map/2, foldl/3]).
-define(API, "https://api.example.com").
-define(PORT, os:getenv("PORT", "8080")).
`
	b := byIdent(sniffErlang(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["map"].ImportSource != "lists" {
		t.Errorf("map import: %+v", b["map"])
	}
}

func TestFSharpSniffer(t *testing.T) {
	const src = `open System
open Other.Module

[<Literal>] let API = "https://api.example.com"
let NAME = "literal"
let PORT = Environment.GetEnvironmentVariable("PORT") |> Option.ofObj |> Option.defaultValue "8080"
`
	b := byIdent(sniffFSharp(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["System"].ImportSource != "System" {
		t.Errorf("System open: %+v", b["System"])
	}
	if b["Module"].ImportSource != "Other.Module" {
		t.Errorf("Module open: %+v", b["Module"])
	}
}

func TestHaskellSniffer(t *testing.T) {
	const src = `module App where
import Data.List
import qualified Data.Map as Map

api = "https://api.example.com"
name = "literal"
`
	b := byIdent(sniffHaskell(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["List"].ImportSource != "Data.List" {
		t.Errorf("List import: %+v", b["List"])
	}
	if b["Map"].ImportSource != "Data.Map" {
		t.Errorf("Map qualified: %+v", b["Map"])
	}
}

func TestNimSniffer(t *testing.T) {
	const src = `import strutils, os
import other/mod

const API* = "https://api.example.com"
let NAME = "literal"
let PORT = getEnv("PORT", "8080")
`
	b := byIdent(sniffNim(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["strutils"].ImportSource != "strutils" {
		t.Errorf("strutils: %+v", b["strutils"])
	}
	if b["mod"].ImportSource != "other/mod" {
		t.Errorf("mod path: %+v", b["mod"])
	}
}

func TestOCamlSniffer(t *testing.T) {
	const src = `open Lwt
open Other.Module

let api = "https://api.example.com"
let name : string = "literal"
let port = Option.value (Sys.getenv_opt "PORT") ~default:"8080"
`
	b := byIdent(sniffOCaml(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["port"].Value != "8080" || b["port"].EnvVar != "PORT" {
		t.Errorf("port: %+v", b["port"])
	}
	if b["Lwt"].ImportSource != "Lwt" {
		t.Errorf("Lwt open: %+v", b["Lwt"])
	}
}

func TestReasonMLSniffer(t *testing.T) {
	const src = `open Other.Module;
let api = "https://api.example.com";
let name: string = "literal";
`
	b := byIdent(sniffReasonML(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["Module"].ImportSource != "Other.Module" {
		t.Errorf("Module open: %+v", b["Module"])
	}
}

func TestReScriptSniffer(t *testing.T) {
	const src = `open Belt
let api = "https://api.example.com"
let name: string = "literal"
let port = Js.Dict.get(Node.Process.process["env"], "PORT")->Belt.Option.getWithDefault("8080")
`
	b := byIdent(sniffReScript(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["port"].Value != "8080" || b["port"].EnvVar != "PORT" {
		t.Errorf("port: %+v", b["port"])
	}
	if b["Belt"].ImportSource != "Belt" {
		t.Errorf("Belt open: %+v", b["Belt"])
	}
}

func TestSMLSniffer(t *testing.T) {
	const src = `open Other.Module
val api = "https://api.example.com"
val name : string = "literal"
val port = case OS.Process.getEnv "PORT" of SOME s => s | NONE => "8080"
`
	b := byIdent(sniffSML(src))
	if b["api"].Value != "https://api.example.com" {
		t.Errorf("api: %+v", b["api"])
	}
	if b["name"].Value != "literal" {
		t.Errorf("name: %+v", b["name"])
	}
	if b["port"].Value != "8080" || b["port"].EnvVar != "PORT" {
		t.Errorf("port: %+v", b["port"])
	}
	if b["Module"].ImportSource != "Other.Module" {
		t.Errorf("Module open: %+v", b["Module"])
	}
}

func TestSoliditySniffer(t *testing.T) {
	const src = `import "./other.sol";
import {Util, Helper as H} from "./util.sol";

string constant API = "https://api.example.com";
string public constant NAME = "literal";
`
	b := byIdent(sniffSolidity(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["other"].ImportSource != "./other.sol" {
		t.Errorf("other import: %+v", b["other"])
	}
	if b["Util"].ImportSource != "./util.sol" {
		t.Errorf("Util import: %+v", b["Util"])
	}
	if b["H"].ImportSource != "./util.sol" {
		t.Errorf("H aliased: %+v", b["H"])
	}
}

func TestZigSniffer(t *testing.T) {
	const src = `const std = @import("std");
pub const API = "https://api.example.com";
const NAME: []const u8 = "literal";
`
	b := byIdent(sniffZig(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", b["NAME"])
	}
	if b["std"].ImportSource != "std" {
		t.Errorf("std @import: %+v", b["std"])
	}
}

func TestMarkupScriptSniffer(t *testing.T) {
	// Vue / Svelte / Astro share the same dispatcher; one fixture exercises
	// the JSTS sniffer reuse and the line-offset adjustment.
	const src = `<template>
  <div>hello</div>
</template>
<script lang="ts">
const API = "https://api.example.com";
const PORT = process.env.PORT ?? "8080";
import { Util } from "./util";
</script>`
	b := byIdent(sniffMarkupScript(src))
	if b["API"].Value != "https://api.example.com" {
		t.Errorf("API: %+v", b["API"])
	}
	if b["PORT"].Value != "8080" || b["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", b["PORT"])
	}
	if b["Util"].ImportSource != "./util" {
		t.Errorf("Util import: %+v", b["Util"])
	}
	// Line numbers should be offset into the original markup (script body
	// starts on line 4 → API on line 5).
	if b["API"].Line < 4 {
		t.Errorf("API line should be offset to original markup, got %d", b["API"].Line)
	}
}
