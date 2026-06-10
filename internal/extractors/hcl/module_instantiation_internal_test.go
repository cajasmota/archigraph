package hcl

import "testing"

// TestResolveModuleSourceDir unit-tests the path resolution directly (#4657).
func TestResolveModuleSourceDir(t *testing.T) {
	cases := []struct {
		name, source, file, want string
	}{
		{"two-up", "../../modules/worker-service", "envs/prod/main.tf", "modules/worker-service"},
		{"one-up", "../shared", "stacks/app/main.tf", "stacks/shared"},
		{"dot-slash", "./local-mod", "infra/main.tf", "infra/local-mod"},
		{"registry", "terraform-aws-modules/vpc/aws", "envs/dev/main.tf", ""},
		{"git", "git::https://example.com/mod.git", "envs/dev/main.tf", ""},
		{"escapes-repo", "../../../outside", "main.tf", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveModuleSourceDir(tc.source, tc.file); got != tc.want {
				t.Errorf("resolveModuleSourceDir(%q, %q) = %q, want %q", tc.source, tc.file, got, tc.want)
			}
		})
	}
}

// TestEnvFromPath unit-tests env derivation (#4657).
func TestEnvFromPath(t *testing.T) {
	cases := []struct{ path, want string }{
		{"envs/prod/main.tf", "prod"},
		{"infra/environments/staging/main.tf", "staging"},
		{"env/dev/network.tf", "dev"},
		{"modules/worker-service/main.tf", ""},
		{"main.tf", ""},
	}
	for _, tc := range cases {
		if got := envFromPath(tc.path); got != tc.want {
			t.Errorf("envFromPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
