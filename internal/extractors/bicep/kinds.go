package bicep

import "github.com/cajasmota/grafel/internal/types"

// bicepResourceCoarseScope returns the uniform IaC resource_category for an
// Azure resource-provider type (e.g. "Microsoft.Storage/storageAccounts"). It
// now delegates to the ONE shared classifier (types.IaCResourceCategory) so
// Bicep resources carry exactly the same `resource_category` values as
// Terraform / CDK / Pulumi / CFN, making a cross-tool "all datastores" query
// possible (#3549). All Bicep resources stay a single queryable
// SCOPE.InfraResource entity Kind; the finer category is captured as an
// attribute rather than a distinct entity Kind, so existing QualifiedNames and
// DEPENDS_ON edges are unchanged.
func bicepResourceCoarseScope(rpType string) string {
	return types.IaCResourceCategory(rpType)
}
