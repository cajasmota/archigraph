package extractor

import "testing"

func TestIsSyntheticSourceFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Known constant sentinels.
		{ConfigKeySourceFile, true},       // "<config>"
		{ExceptionTypeSourceFile, true},   // "<exception>"
		{ExternalServiceSourceFile, true}, // "<external-service>"
		{TranslationKeySourceFile, true},  // "<translation-key>"
		{TemplateSourceFile, true},        // "<template>"
		// General angle-bracket shape (future synthetic sentinels).
		{"<config>", true},
		{"<generated>", true},
		{"<stdin>", true},
		{"<synthetic>", true},
		{"<anything>", true},
		// Real paths must not match.
		{"src/handler.go", false},
		{"internal/links/string_pass.go", false},
		{"C:\\Users\\me\\app.go", false},
		{"a/b/c.py", false},
		{"", false},
		// Partial / malformed brackets are not sentinels.
		{"<config", false},
		{"config>", false},
		{"<", false},
	}
	for _, c := range cases {
		if got := IsSyntheticSourceFile(c.path); got != c.want {
			t.Errorf("IsSyntheticSourceFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
