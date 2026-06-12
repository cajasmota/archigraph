package coverage

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name, raw, root, want string
	}{
		{"already relative", "src/foo.ts", "", "src/foo.ts"},
		{"leading dotslash", "./src/foo.ts", "", "src/foo.ts"},
		{"absolute strip root", "/home/ci/project/src/foo.ts", "project", "src/foo.ts"},
		{"relative strip root", "packages/web/src/foo.ts", "packages/web", "src/foo.ts"},
		{"windows slashes", "src\\foo.ts", "", "src/foo.ts"},
		{"root with slashes", "/a/b/build/src/foo.ts", "/build/", "src/foo.ts"},
		{"dot collapse", "src/./a/../foo.ts", "", "src/foo.ts"},
		{"no root match", "/abs/src/foo.ts", "nomatch", "abs/src/foo.ts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.raw, c.root); got != c.want {
				t.Errorf("Normalize(%q,%q) = %q, want %q", c.raw, c.root, got, c.want)
			}
		})
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("src/foo.ts", "src/foo.ts") {
		t.Error("identical should match")
	}
	if !samePath("a/b/src/foo.ts", "src/foo.ts") {
		t.Error("suffix should match")
	}
	if samePath("src/foo.ts", "src/bar.ts") {
		t.Error("distinct files must not match")
	}
}
