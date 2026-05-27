package substrate

import "testing"

// bindMap is a tiny test helper that indexes a slice of Binding by Ident
// for ergonomic lookups in the per-language tests below. We define it once
// per file because the T1 tests inline the same idiom — keeping it local
// avoids cross-file test coupling.
func bindMap(bs []Binding) map[string]Binding {
	by := map[string]Binding{}
	for _, b := range bs {
		by[b.Ident] = b
	}
	return by
}

func TestRubySniffer(t *testing.T) {
	const src = `API_URL = "https://api.example.com"
NAME = 'literal'
PORT = ENV.fetch("PORT", "8080")
HOST = ENV.fetch("HOST") { "localhost" }
DB = ENV["DB_URL"] || "postgres://localhost"
require "lib/foo"
require_relative "./bar"
`
	by := bindMap(sniffRuby(src))
	if by["API_URL"].Value != "https://api.example.com" || by["API_URL"].Provenance != ProvenanceLiteral {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["HOST"].Value != "localhost" || by["HOST"].EnvVar != "HOST" {
		t.Errorf("HOST: %+v", by["HOST"])
	}
	if by["DB"].Value != "postgres://localhost" || by["DB"].EnvVar != "DB_URL" {
		t.Errorf("DB: %+v", by["DB"])
	}
	if by["foo"].ImportSource != "lib/foo" || by["foo"].Provenance != ProvenanceCrossFile {
		t.Errorf("foo require: %+v", by["foo"])
	}
	if by["bar"].ImportSource != "./bar" {
		t.Errorf("bar require_relative: %+v", by["bar"])
	}
}

func TestPHPSniffer(t *testing.T) {
	const src = `<?php
const API_URL = "https://api.example.com";
define("DEFINED_NAME", "shh");
$port = "8080";
$db = getenv("DB_URL") ?: "postgres://localhost";
$host = getenv("HOST") ?? "localhost";
use Foo\Bar;
use Foo\Quux as Q;
`
	by := bindMap(sniffPHP(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["DEFINED_NAME"].Value != "shh" {
		t.Errorf("DEFINED_NAME: %+v", by["DEFINED_NAME"])
	}
	if by["port"].Value != "8080" {
		t.Errorf("port: %+v", by["port"])
	}
	if by["db"].Value != "postgres://localhost" || by["db"].EnvVar != "DB_URL" {
		t.Errorf("db: %+v", by["db"])
	}
	if by["host"].Value != "localhost" || by["host"].EnvVar != "HOST" {
		t.Errorf("host: %+v", by["host"])
	}
	if by["Bar"].ImportSource != "Foo" || by["Bar"].Provenance != ProvenanceCrossFile {
		t.Errorf("Bar use: %+v", by["Bar"])
	}
	if by["Q"].ImportSource != "Foo" {
		t.Errorf("Q aliased use: %+v", by["Q"])
	}
}

func TestRustSniffer(t *testing.T) {
	const src = `const API_URL: &str = "https://api.example.com";
pub static NAME: &'static str = "literal";
static PORT: String = env::var("PORT").unwrap_or("8080".into());
static HOST: String = env::var("HOST").unwrap_or_else(|_| "localhost".into());
use foo::Bar;
use foo::{X, Y as Z};
`
	by := bindMap(sniffRust(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["HOST"].Value != "localhost" || by["HOST"].EnvVar != "HOST" {
		t.Errorf("HOST: %+v", by["HOST"])
	}
	if by["Bar"].ImportSource != "foo" || by["Bar"].Provenance != ProvenanceCrossFile {
		t.Errorf("Bar use: %+v", by["Bar"])
	}
	if by["X"].ImportSource != "foo::X" {
		t.Errorf("X braced use: %+v", by["X"])
	}
	if by["Z"].ImportSource != "foo::Y" {
		t.Errorf("Z aliased use: %+v", by["Z"])
	}
}

func TestCSharpSniffer(t *testing.T) {
	const src = `using System;
using Sys = System.Foo;

public class Config {
    public const string API_URL = "https://api.example.com";
    private static readonly string SECRET = "shh";
    public static readonly string DB_URL = Environment.GetEnvironmentVariable("DB_URL") ?? "jdbc:postgresql://localhost/x";
}
`
	by := bindMap(sniffCSharp(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["SECRET"].Value != "shh" {
		t.Errorf("SECRET: %+v", by["SECRET"])
	}
	if by["DB_URL"].Value != "jdbc:postgresql://localhost/x" || by["DB_URL"].EnvVar != "DB_URL" {
		t.Errorf("DB_URL: %+v", by["DB_URL"])
	}
	if by["System"].Provenance != ProvenanceCrossFile {
		t.Errorf("System using: %+v", by["System"])
	}
	if by["Sys"].ImportSource != "System.Foo" {
		t.Errorf("Sys aliased using: %+v", by["Sys"])
	}
}

func TestKotlinSniffer(t *testing.T) {
	const src = `package com.example
import com.other.Util
import com.other.Helper as H

const val API_URL = "https://api.example.com"
val NAME: String = "literal"
val PORT = System.getenv("PORT") ?: "8080"
`
	by := bindMap(sniffKotlin(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["Util"].ImportSource != "com.other" {
		t.Errorf("Util import: %+v", by["Util"])
	}
	if by["H"].ImportSource != "com.other.Helper" {
		t.Errorf("H aliased import: %+v", by["H"])
	}
}

func TestElixirSniffer(t *testing.T) {
	const src = `defmodule MyApp.Config do
  @api_url "https://api.example.com"
  @name 'literal'
  @port System.get_env("PORT", "8080")
  @host System.get_env("HOST") || "localhost"
  alias Foo.Bar
  alias Foo.{X, Y}
  alias Foo.Other, as: O
end
`
	by := bindMap(sniffElixir(src))
	if by["api_url"].Value != "https://api.example.com" {
		t.Errorf("api_url: %+v", by["api_url"])
	}
	if by["name"].Value != "literal" {
		t.Errorf("name: %+v", by["name"])
	}
	if by["port"].Value != "8080" || by["port"].EnvVar != "PORT" {
		t.Errorf("port: %+v", by["port"])
	}
	if by["host"].Value != "localhost" || by["host"].EnvVar != "HOST" {
		t.Errorf("host: %+v", by["host"])
	}
	if by["Bar"].ImportSource != "Foo" || by["Bar"].Provenance != ProvenanceCrossFile {
		t.Errorf("Bar alias: %+v", by["Bar"])
	}
	if by["X"].ImportSource != "Foo.X" {
		t.Errorf("X braced alias: %+v", by["X"])
	}
	if by["O"].ImportSource != "Foo.Other" {
		t.Errorf("O rebinding alias: %+v", by["O"])
	}
}

func TestScalaSniffer(t *testing.T) {
	const src = `package com.example
import com.other.Util
import com.other.{A, B => Bz}

object Config {
  val API_URL = "https://api.example.com"
  final val NAME: String = "literal"
  val PORT = sys.env.getOrElse("PORT", "8080")
}
`
	by := bindMap(sniffScala(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["Util"].ImportSource != "com.other" {
		t.Errorf("Util import: %+v", by["Util"])
	}
	if by["A"].ImportSource != "com.other.A" {
		t.Errorf("A braced import: %+v", by["A"])
	}
	if by["Bz"].ImportSource != "com.other.B" {
		t.Errorf("Bz rebinding import: %+v", by["Bz"])
	}
}

func TestCCPPSniffer(t *testing.T) {
	const src = `#include "foo/bar.h"
#include <stdio.h>
#define API_URL "https://api.example.com"

const char* NAME = "literal";
static const char* SECRET = "shh";
const char ARR[] = "arrform";
const char* DB_URL = getenv("DB_URL") ? getenv("DB_URL") : "postgres://localhost";
`
	by := bindMap(sniffCCPP(src))
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL #define: %+v", by["API_URL"])
	}
	if by["NAME"].Value != "literal" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["SECRET"].Value != "shh" {
		t.Errorf("SECRET: %+v", by["SECRET"])
	}
	if by["ARR"].Value != "arrform" {
		t.Errorf("ARR array form: %+v", by["ARR"])
	}
	if by["DB_URL"].Value != "postgres://localhost" || by["DB_URL"].EnvVar != "DB_URL" {
		t.Errorf("DB_URL: %+v", by["DB_URL"])
	}
	if by["bar"].ImportSource != "foo/bar.h" || by["bar"].Provenance != ProvenanceCrossFile {
		t.Errorf("bar include: %+v", by["bar"])
	}
	if by["stdio"].ImportSource != "stdio.h" {
		t.Errorf("stdio include: %+v", by["stdio"])
	}
}
