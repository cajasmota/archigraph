// Value-asserting tests for the AWS CDK JVM (Java) / Go / .NET (C#) language
// bindings (cdk_edges_jvm_go_net.go) — #3550, epic #3512.
//
// Each test asserts the EXACT extracted shape (specific construct → LogicalId →
// resource_category, and a specific DEPENDS_ON edge), not len>0.
package engine

import "testing"

// TestCDKJava_BucketLambdaGrant: Java `new`-form + builder-form constructs and a
// camelCase grant edge. Asserts:
//   - Bucket "Assets" → resource_category "storage"
//   - Function "Fn"   → resource_category "function" (built via Builder.create)
//   - grantRead → DEPENDS_ON Fn → Assets
func TestCDKJava_BucketLambdaGrant(t *testing.T) {
	src := `package com.example;

import software.amazon.awscdk.Stack;
import software.amazon.awscdk.services.s3.Bucket;
import software.amazon.awscdk.services.s3.BucketProps;
import software.amazon.awscdk.services.lambda.Function;

public class DataStack extends Stack {
    public DataStack(final Construct scope, final String id) {
        super(scope, id);

        Bucket bucket = new Bucket(this, "Assets", BucketProps.builder()
            .versioned(true)
            .build());

        Function fn = Function.Builder.create(this, "Fn")
            .runtime(Runtime.JAVA_17)
            .handler("Handler")
            .build();

        bucket.grantRead(fn);
    }
}
`
	ents, rels := runCDKDetect(t, "java", "src/main/java/com/example/DataStack.java", src)

	bucket := cdkResourceByName(ents, "Assets")
	if bucket == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Assets', got %+v", ents)
	}
	if bucket.props["construct_type"] != "s3.Bucket" {
		t.Errorf("Assets construct_type = %q, want s3.Bucket", bucket.props["construct_type"])
	}
	if bucket.props["resource_category"] != "storage" {
		t.Errorf("Assets resource_category = %q, want storage", bucket.props["resource_category"])
	}
	if bucket.props["iac_tool"] != "aws-cdk" {
		t.Errorf("Assets iac_tool = %q, want aws-cdk", bucket.props["iac_tool"])
	}

	fn := cdkResourceByName(ents, "Fn")
	if fn == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Fn' (Builder form), got %+v", ents)
	}
	if fn.props["construct_type"] != "lambda.Function" {
		t.Errorf("Fn construct_type = %q, want lambda.Function", fn.props["construct_type"])
	}
	if fn.props["resource_category"] != "function" {
		t.Errorf("Fn resource_category = %q, want function", fn.props["resource_category"])
	}

	if !hasDependsOn(rels, "SCOPE.InfraResource:Fn", "SCOPE.InfraResource:Assets") {
		t.Fatalf("expected DEPENDS_ON Fn→Assets (grantRead), got %+v", relsByKind(rels, "DEPENDS_ON"))
	}
}

// TestCDKGo_BucketLambdaGrant: Go factory-form constructs (`pkg.NewType(...,
// jsii.String("Id"), ...)`) and a PascalCase grant. Asserts the factory→type
// New-strip mapping feeds the classifier, and the grant edge direction.
//   - awss3.NewBucket → type awss3.Bucket → category "storage"
//   - awslambda.NewFunction → awslambda.Function → category "function"
//   - GrantInvoke → DEPENDS_ON Fn → Assets
func TestCDKGo_BucketLambdaGrant(t *testing.T) {
	src := `package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func NewDataStack(scope constructs.Construct, id string) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, nil)

	bucket := awss3.NewBucket(stack, jsii.String("Assets"), &awss3.BucketProps{
		Versioned: jsii.Bool(true),
	})

	fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_GO_1_X(),
	})

	bucket.GrantInvoke(fn, nil)

	return stack
}
`
	ents, rels := runCDKDetect(t, "go", "stack.go", src)

	bucket := cdkResourceByName(ents, "Assets")
	if bucket == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Assets', got %+v", ents)
	}
	if bucket.props["construct_type"] != "awss3.Bucket" {
		t.Errorf("Assets construct_type = %q, want awss3.Bucket (New-strip)", bucket.props["construct_type"])
	}
	if bucket.props["resource_category"] != "storage" {
		t.Errorf("Assets resource_category = %q, want storage", bucket.props["resource_category"])
	}

	fn := cdkResourceByName(ents, "Fn")
	if fn == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Fn', got %+v", ents)
	}
	if fn.props["construct_type"] != "awslambda.Function" {
		t.Errorf("Fn construct_type = %q, want awslambda.Function", fn.props["construct_type"])
	}
	if fn.props["resource_category"] != "function" {
		t.Errorf("Fn resource_category = %q, want function", fn.props["resource_category"])
	}

	if !hasDependsOn(rels, "SCOPE.InfraResource:Fn", "SCOPE.InfraResource:Assets") {
		t.Fatalf("expected DEPENDS_ON Fn→Assets (GrantInvoke), got %+v", relsByKind(rels, "DEPENDS_ON"))
	}
}

// TestCDKCSharp_BucketTableGrant: C# `new`-form constructs + PascalCase grant.
// Uses a DynamoDB table to assert the "datastore" category distinctly from
// storage/function.
//   - Bucket "Assets" → "storage"
//   - Table  "Orders" → "datastore"
//   - table.GrantReadData(fn-less)… here grant from table to a function var
func TestCDKCSharp_BucketTableGrant(t *testing.T) {
	src := `using Amazon.CDK;
using Amazon.CDK.AWS.S3;
using Amazon.CDK.AWS.DynamoDB;
using Amazon.CDK.AWS.Lambda;

namespace Example
{
    public class DataStack : Stack
    {
        public DataStack(Construct scope, string id) : base(scope, id)
        {
            var bucket = new Bucket(this, "Assets", new BucketProps { Versioned = true });

            var table = new Table(this, "Orders", new TableProps { });

            var fn = new Function(this, "Fn", new FunctionProps { });

            table.GrantReadData(fn);
        }
    }
}
`
	ents, rels := runCDKDetect(t, "csharp", "DataStack.cs", src)

	bucket := cdkResourceByName(ents, "Assets")
	if bucket == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Assets', got %+v", ents)
	}
	if bucket.props["construct_type"] != "s3.Bucket" {
		t.Errorf("Assets construct_type = %q, want s3.Bucket", bucket.props["construct_type"])
	}
	if bucket.props["resource_category"] != "storage" {
		t.Errorf("Assets resource_category = %q, want storage", bucket.props["resource_category"])
	}

	table := cdkResourceByName(ents, "Orders")
	if table == nil {
		t.Fatalf("expected SCOPE.InfraResource 'Orders', got %+v", ents)
	}
	if table.props["construct_type"] != "dynamodb.Table" {
		t.Errorf("Orders construct_type = %q, want dynamodb.Table", table.props["construct_type"])
	}
	if table.props["resource_category"] != "datastore" {
		t.Errorf("Orders resource_category = %q, want datastore", table.props["resource_category"])
	}

	// table.GrantReadData(fn) → fn DEPENDS_ON table.
	if !hasDependsOn(rels, "SCOPE.InfraResource:Fn", "SCOPE.InfraResource:Orders") {
		t.Fatalf("expected DEPENDS_ON Fn→Orders (GrantReadData), got %+v", relsByKind(rels, "DEPENDS_ON"))
	}
}

// TestCDKJVMGoNet_NonCDKFileIgnored: a Go/Java/C# file with no CDK import emits
// nothing (the pre-filter guards the generic `new X(this,"id")` idiom).
func TestCDKJVMGoNet_NonCDKFileIgnored(t *testing.T) {
	goSrc := `package main
func main() {
	bucket := foo.NewBucket(stack, jsii.String("Assets"), nil)
	_ = bucket
}
`
	ents, _ := runCDKDetect(t, "go", "main.go", goSrc)
	if len(ents) != 0 {
		t.Fatalf("non-CDK Go file should emit no resources, got %+v", ents)
	}
}

// hasDependsOn reports whether a DEPENDS_ON edge from→to exists.
func hasDependsOn(rels []relResult, from, to string) bool {
	for _, r := range relsByKind(rels, "DEPENDS_ON") {
		if r.from == from && r.to == to {
			return true
		}
	}
	return false
}
