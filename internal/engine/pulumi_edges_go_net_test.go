// Value-asserting tests for the Pulumi Go / .NET (C#) language bindings
// (pulumi_edges_go_net.go) — #3550, epic #3512.
//
// Each test asserts the EXACT extracted shape: a specific resource constructor →
// logical name → resource_category, and a specific DEPENDS_ON edge (a bucket's
// policy DEPENDS_ON the bucket), not len>0.
package engine

import "testing"

// TestPulumiGo_BucketPolicyDependsOn: Go Pulumi resources with an explicit
// DependsOn and an output reference. Asserts:
//   - s3.NewBucket → type s3.Bucket → category "storage"
//   - the policy resource DEPENDS_ON the bucket (both via output ref and the
//     explicit DependsOn list resolve to the same edge).
func TestPulumiGo_BucketPolicyDependsOn(t *testing.T) {
	src := `package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		bucket, err := s3.NewBucket(ctx, "assets", &s3.BucketArgs{
			Acl: pulumi.String("private"),
		})
		if err != nil {
			return err
		}

		_, err = s3.NewBucketPolicy(ctx, "assets-policy", &s3.BucketPolicyArgs{
			Bucket: bucket.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{bucket}))
		if err != nil {
			return err
		}
		return nil
	})
}
`
	ents, rels := runPulumiDetect(t, "go", "main.go", src)

	bucket := pulumiResourceByName(ents, "assets")
	if bucket == nil {
		t.Fatalf("expected SCOPE.InfraResource 'assets', got %+v", ents)
	}
	if bucket.props["construct_type"] != "s3.Bucket" {
		t.Errorf("assets construct_type = %q, want s3.Bucket (New-strip)", bucket.props["construct_type"])
	}
	if bucket.props["resource_category"] != "storage" {
		t.Errorf("assets resource_category = %q, want storage", bucket.props["resource_category"])
	}
	if bucket.props["iac_tool"] != "pulumi" {
		t.Errorf("assets iac_tool = %q, want pulumi", bucket.props["iac_tool"])
	}

	policy := pulumiResourceByName(ents, "assets-policy")
	if policy == nil {
		t.Fatalf("expected SCOPE.InfraResource 'assets-policy', got %+v", ents)
	}

	// policy DEPENDS_ON bucket (explicit DependsOn and/or output ref).
	if !hasDependsOn(rels, "SCOPE.InfraResource:assets-policy", "SCOPE.InfraResource:assets") {
		t.Fatalf("expected DEPENDS_ON assets-policy→assets, got %+v", relsByKind(rels, "DEPENDS_ON"))
	}
}

// TestPulumiCSharp_BucketPolicyDependsOn: C# Pulumi resources with an explicit
// CustomResourceOptions.DependsOn and an output reference. Asserts the resource
// types/categories and the policy→bucket dependency edge.
func TestPulumiCSharp_BucketPolicyDependsOn(t *testing.T) {
	src := `using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var bucket = new Aws.S3.Bucket("assets", new Aws.S3.BucketArgs
        {
            Acl = "private",
        });

        var policy = new Aws.S3.BucketPolicy("assets-policy", new Aws.S3.BucketPolicyArgs
        {
            Bucket = bucket.Id,
        }, new CustomResourceOptions { DependsOn = { bucket } });
    }
}
`
	ents, rels := runPulumiDetect(t, "csharp", "MyStack.cs", src)

	bucket := pulumiResourceByName(ents, "assets")
	if bucket == nil {
		t.Fatalf("expected SCOPE.InfraResource 'assets', got %+v", ents)
	}
	if bucket.props["construct_type"] != "Aws.S3.Bucket" {
		t.Errorf("assets construct_type = %q, want Aws.S3.Bucket", bucket.props["construct_type"])
	}
	if bucket.props["resource_category"] != "storage" {
		t.Errorf("assets resource_category = %q, want storage", bucket.props["resource_category"])
	}

	policy := pulumiResourceByName(ents, "assets-policy")
	if policy == nil {
		t.Fatalf("expected SCOPE.InfraResource 'assets-policy', got %+v", ents)
	}

	if !hasDependsOn(rels, "SCOPE.InfraResource:assets-policy", "SCOPE.InfraResource:assets") {
		t.Fatalf("expected DEPENDS_ON assets-policy→assets, got %+v", relsByKind(rels, "DEPENDS_ON"))
	}
}

// TestPulumiGoNet_NonPulumiFileIgnored: a Go file with no Pulumi import emits
// nothing (the pre-filter guards the generic factory idiom).
func TestPulumiGoNet_NonPulumiFileIgnored(t *testing.T) {
	goSrc := `package main
func main() {
	x, _ := s3.NewBucket(ctx, "assets", nil)
	_ = x
}
`
	ents, _ := runPulumiDetect(t, "go", "main.go", goSrc)
	if len(ents) != 0 {
		t.Fatalf("non-Pulumi Go file should emit no resources, got %+v", ents)
	}
}
