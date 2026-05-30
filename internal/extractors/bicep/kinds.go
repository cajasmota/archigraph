package bicep

import "strings"

// bicepResourceCoarseScope maps an Azure resource-provider type
// (e.g. "Microsoft.Storage/storageAccounts") to a coarse architectural scope
// label stored on the resource entity's Metadata. This mirrors the CDK
// extractor's resource_scope attribute (internal/engine/cdk_edges.go): all
// Bicep resources stay a single queryable SCOPE.InfraResource entity Kind, with
// the finer datastore/queue/compute classification captured as an attribute
// rather than a distinct entity Kind.
//
// The classification is deliberately conservative — it keys off the well-known
// Azure resource-provider namespaces and resource-type segments.
func bicepResourceCoarseScope(rpType string) string {
	t := strings.ToLower(rpType)
	switch {
	case strings.Contains(t, "microsoft.storage/") ||
		strings.Contains(t, "microsoft.sql/") ||
		strings.Contains(t, "microsoft.documentdb/") ||
		strings.Contains(t, "microsoft.dbforpostgresql/") ||
		strings.Contains(t, "microsoft.dbformysql/") ||
		strings.Contains(t, "microsoft.cache/") ||
		strings.Contains(t, "database") ||
		strings.Contains(t, "cosmosdb"):
		return "datastore"
	case strings.Contains(t, "microsoft.servicebus/") ||
		strings.Contains(t, "microsoft.eventhub/") ||
		strings.Contains(t, "microsoft.eventgrid/") ||
		strings.Contains(t, "/queues") ||
		strings.Contains(t, "/topics"):
		return "queue"
	case strings.Contains(t, "microsoft.web/sites") ||
		strings.Contains(t, "microsoft.compute/") ||
		strings.Contains(t, "microsoft.containerservice/") ||
		strings.Contains(t, "microsoft.app/"):
		return "compute"
	case strings.Contains(t, "microsoft.network/"):
		return "network"
	default:
		return "service"
	}
}
