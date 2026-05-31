package bicep_test

import "testing"

// TestBicep_ResourceCategoryStamp proves the Bicep extractor carries the SAME
// cross-tool resource_category property as the other IaC tools (#3549):
// a Service Bus queue → queue, a Function App → function, a SQL database →
// datastore. Value-asserting on the exact category, not len>0.
func TestBicep_ResourceCategoryStamp(t *testing.T) {
	const src = `
resource sbQueue 'Microsoft.ServiceBus/namespaces/queues@2022-10-01' = {
  name: 'jobs'
}

resource fnApp 'Microsoft.Web/sites/functions@2022-09-01' = {
  name: 'worker'
}

resource sqlDb 'Microsoft.Sql/servers/databases@2022-05-01' = {
  name: 'orders'
}
`
	recs := extractBicep(t, src, "main.bicep")

	cases := []struct {
		name string
		want string
	}{
		{"sbQueue", "queue"},
		{"fnApp", "function"},
		{"sqlDb", "datastore"},
	}
	for _, c := range cases {
		r := findByName(recs, c.name)
		if r == nil {
			t.Fatalf("resource %q not found", c.name)
		}
		if got := r.Metadata["resource_category"]; got != c.want {
			t.Errorf("%s resource_category = %v, want %q", c.name, got, c.want)
		}
		if got := r.Metadata["resource_scope"]; got != c.want {
			t.Errorf("%s resource_scope = %v, want %q (alias)", c.name, got, c.want)
		}
	}
}
