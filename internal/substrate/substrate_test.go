package substrate

import "testing"

func TestLanguageForPath(t *testing.T) {
	cases := map[string]string{
		"a.ts":     "jsts",
		"a.tsx":    "jsts",
		"a.js":     "jsts",
		"a.jsx":    "jsts",
		"a.mjs":    "jsts",
		"a.cjs":    "jsts",
		"a.py":     "python",
		"a.pyi":    "python",
		"a.java":   "java",
		"a.go":     "go",
		"a.rs":     "",
		"":         "",
		"a.txt":    "",
		"foo/x.go": "go",
	}
	for in, want := range cases {
		if got := LanguageForPath(in); got != want {
			t.Errorf("LanguageForPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRegisterAndSnifferFor(t *testing.T) {
	for _, lang := range []string{"jsts", "python", "java", "go"} {
		if SnifferFor(lang) == nil {
			t.Errorf("expected sniffer registered for %q", lang)
		}
	}
}

func TestJSTSSnifferLiterals(t *testing.T) {
	const src = `
const API_URL = "https://api.example.com";
let other = 'literal';
export const FOO = "bar";
const X = process.env.MY_VAR ?? "fallback";
const Y = import.meta.env.VITE_X || 'def';
import { Z, A as B } from "./shared";
`
	got := sniffJSTS(src)
	want := map[string]string{
		"API_URL": "https://api.example.com",
		"other":   "literal",
		"FOO":     "bar",
		"X":       "fallback",
		"Y":       "def",
	}
	bindByName := map[string]Binding{}
	for _, b := range got {
		bindByName[b.Ident] = b
	}
	for name, val := range want {
		b, ok := bindByName[name]
		if !ok {
			t.Errorf("missing binding %q", name)
			continue
		}
		if b.Value != val {
			t.Errorf("binding %q value = %q, want %q", name, b.Value, val)
		}
	}
	if bindByName["X"].Provenance != ProvenanceEnvFallback {
		t.Errorf("X provenance = %q, want env_fallback", bindByName["X"].Provenance)
	}
	if bindByName["X"].EnvVar != "MY_VAR" {
		t.Errorf("X env_var = %q, want MY_VAR", bindByName["X"].EnvVar)
	}
	// Import bindings:
	if bindByName["Z"].Provenance != ProvenanceCrossFile || bindByName["Z"].ImportSource != "./shared" {
		t.Errorf("Z import: %+v", bindByName["Z"])
	}
	if bindByName["B"].ImportSource != "./shared" {
		t.Errorf("B (aliased) import: %+v", bindByName["B"])
	}
}

func TestPythonSnifferLiteralsAndEnv(t *testing.T) {
	const src = `import os
API_URL = "https://api.example.com"
PORT = os.getenv("PORT", "8080")
DB_URL = os.environ.get("DB_URL", 'postgres://localhost')
HOST = os.getenv("HOST") or "localhost"
TIMEOUT: int = 30
NAME: str = "x"
from .shared import Foo, Bar as Baz
`
	got := sniffPython(src)
	by := map[string]Binding{}
	for _, b := range got {
		by[b.Ident] = b
	}
	if by["API_URL"].Value != "https://api.example.com" || by["API_URL"].Provenance != ProvenanceLiteral {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["DB_URL"].Value != "postgres://localhost" || by["DB_URL"].EnvVar != "DB_URL" {
		t.Errorf("DB_URL: %+v", by["DB_URL"])
	}
	if by["HOST"].Value != "localhost" || by["HOST"].EnvVar != "HOST" {
		t.Errorf("HOST: %+v", by["HOST"])
	}
	if by["NAME"].Value != "x" {
		t.Errorf("NAME: %+v", by["NAME"])
	}
	if by["Foo"].Provenance != ProvenanceCrossFile || by["Foo"].ImportSource != ".shared" {
		t.Errorf("Foo import: %+v", by["Foo"])
	}
	if by["Baz"].ImportSource != ".shared" {
		t.Errorf("Baz aliased: %+v", by["Baz"])
	}
}

func TestJavaSnifferLiteralsAndEnv(t *testing.T) {
	const src = `package com.example;
import com.other.Util;
import static com.other.Helper.HELP;

public class Config {
    public static final String API_URL = "https://api.example.com";
    private static final String SECRET = "shh";
    public static final String DB_URL = System.getenv("DB_URL") != null ? System.getenv("DB_URL") : "jdbc:postgresql://localhost/x";
    public static final String PORT = Optional.ofNullable(System.getenv("PORT")).orElse("8080");
}
`
	got := sniffJava(src)
	by := map[string]Binding{}
	for _, b := range got {
		by[b.Ident] = b
	}
	if by["API_URL"].Value != "https://api.example.com" {
		t.Errorf("API_URL: %+v", by["API_URL"])
	}
	if by["DB_URL"].Value != "jdbc:postgresql://localhost/x" || by["DB_URL"].EnvVar != "DB_URL" {
		t.Errorf("DB_URL: %+v", by["DB_URL"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["Util"].ImportSource != "com.other" {
		t.Errorf("Util import: %+v", by["Util"])
	}
	if by["HELP"].ImportSource != "com.other.Helper" {
		t.Errorf("HELP static import: %+v", by["HELP"])
	}
}

func TestGoSnifferLiteralsAndEnv(t *testing.T) {
	const src = `package main

import (
	"cmp"
	"os"
	alias "github.com/x/y"
)

import "fmt"

const APIURL = "https://api.example.com"
var DB string = "postgres://localhost"
var PORT = cmp.Or(os.Getenv("PORT"), "8080")
`
	got := sniffGo(src)
	by := map[string]Binding{}
	for _, b := range got {
		by[b.Ident] = b
	}
	if by["APIURL"].Value != "https://api.example.com" {
		t.Errorf("APIURL: %+v", by["APIURL"])
	}
	if by["DB"].Value != "postgres://localhost" {
		t.Errorf("DB: %+v", by["DB"])
	}
	if by["PORT"].Value != "8080" || by["PORT"].EnvVar != "PORT" {
		t.Errorf("PORT: %+v", by["PORT"])
	}
	if by["cmp"].ImportSource != "cmp" {
		t.Errorf("cmp import: %+v", by["cmp"])
	}
	if by["alias"].ImportSource != "github.com/x/y" {
		t.Errorf("aliased import: %+v", by["alias"])
	}
	if by["fmt"].ImportSource != "fmt" {
		t.Errorf("single-form import: %+v", by["fmt"])
	}
}
