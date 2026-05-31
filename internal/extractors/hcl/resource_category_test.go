// Value-asserting test for the Terraform resource_category stamp — #3549.
// Proves the HCL extractor now carries the SAME cross-tool category property
// the other IaC tools emit: aws_db_instance → datastore, aws_sqs_queue → queue,
// aws_lambda_function → function.
package hcl_test

import "testing"

func TestTerraform_ResourceCategoryStamp(t *testing.T) {
	src := `
resource "aws_db_instance" "primary" {}
resource "aws_sqs_queue" "jobs" {}
resource "aws_lambda_function" "worker" {}
`
	records, err := extractHCL(src, "main.tf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := []struct {
		name string
		want string
	}{
		{"primary", "datastore"},
		{"jobs", "queue"},
		{"worker", "function"},
	}
	for _, c := range cases {
		r := findBySubtypeAndName(records, "resource", c.name)
		if r == nil {
			t.Fatalf("resource %q not found", c.name)
		}
		if got := r.Metadata["resource_category"]; got != c.want {
			t.Errorf("resource %q resource_category = %v, want %q", c.name, got, c.want)
		}
	}
}
