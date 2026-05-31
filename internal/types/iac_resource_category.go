package types

import "strings"

// IaC resource categorization — the ONE shared classifier used by every IaC
// extractor/detector in archigraph (Terraform/HCL, AWS CDK TS+JS+Python,
// Pulumi TS+JS+Python, CloudFormation/SAM, and Azure Bicep). #3549, epic #3512.
//
// # The consistency model (decided, applied everywhere)
//
// Historically each IaC tool classified resources its own way, so a cross-tool
// query like "show me all the datastores" was impossible:
//
//   - CloudFormation mapped AWS::* types to coarse SCOPE.Datastore /
//     SCOPE.Queue / SCOPE.ServerlessFunction entity Kinds.
//   - CDK + Pulumi emitted SCOPE.InfraResource with a private, three-valued
//     `resource_scope` property (service / datastore / queue) computed by a
//     per-tool helper.
//   - Terraform emitted SCOPE.Component/resource with only a `resource_type`,
//     no semantic typing at all.
//   - Bicep emitted SCOPE.InfraResource with its own `resource_scope` helper.
//
// The unifying decision: a single fine-grained `resource_category` PROPERTY is
// the cross-tool join key. Every IaC resource entity, regardless of tool or
// source language, now carries `resource_category` computed by the one
// IaCResourceCategory function below. A query that wants "all datastores"
// filters on `resource_category == datastore` and gets Terraform aws_db_instance,
// CDK dynamodb.Table, Pulumi aws.rds.Instance, CFN AWS::RDS::DBInstance and Bicep
// Microsoft.Sql/servers alike.
//
// We deliberately do NOT force a single entity Kind on every tool:
//
//   - The semantic Kinds CloudFormation already emits (SCOPE.Datastore /
//     SCOPE.Queue / SCOPE.ServerlessFunction) are preserved — they are derived
//     from the same category via IaCKindForCategory so they can never diverge
//     from the property.
//   - CDK / Pulumi / Bicep keep SCOPE.InfraResource and Terraform keeps
//     SCOPE.Component/resource, because their QualifiedNames and the
//     DEPENDS_ON/USES edges already in the graph are keyed off those Kinds;
//     changing the Kind would break edge resolution. The `resource_category`
//     property gives uniform cross-tool semantics WITHOUT that risk.
//
// So: ONE classifier, ONE property name, applied to ALL FOUR tools; the CFN
// entity Kind stays aligned because it is derived from the same classifier.

// IaC resource category constants — the closed set returned by
// IaCResourceCategory. "other" is the fallback for anything unrecognised.
const (
	IaCCategoryDatastore = "datastore"
	IaCCategoryQueue     = "queue"
	IaCCategoryTopic     = "topic"
	IaCCategoryStream    = "stream"
	IaCCategoryFunction  = "function"
	IaCCategoryCache     = "cache"
	IaCCategorySecret    = "secret"
	IaCCategoryNetwork   = "network"
	IaCCategoryCompute   = "compute"
	IaCCategoryStorage   = "storage"
	IaCCategoryOther     = "other"
)

// IaCResourceCategory maps an IaC resource type string — in ANY of the tool
// dialects below — to one of the IaCCategory* constants. It is provider-aware
// and matches case-insensitively on substrings so it tolerates the differing
// shapes the tools use for the "same" resource:
//
//   - Terraform / OpenTofu:  aws_db_instance, azurerm_storage_account, google_sql_database_instance
//   - Azure Bicep:           Microsoft.Sql/servers, Microsoft.ServiceBus/namespaces/queues
//   - AWS CDK construct type: s3.Bucket, dynamodb.Table, sqs.Queue, lambda.Function, CfnDBInstance
//   - Pulumi resource type:   aws.s3.Bucket, aws.rds.Instance, aws.sns.Topic, azure.storage.Account
//   - CloudFormation/SAM:     AWS::RDS::DBInstance, AWS::SQS::Queue, AWS::Lambda::Function
//
// Ordering matters: more specific categories (cache/secret/stream/topic/queue/
// function) are tested before the broad datastore/storage/compute/network
// buckets so e.g. an ElastiCache resource is `cache`, not `datastore`, and an
// SNS topic is `topic`, not `queue`.
func IaCResourceCategory(typeString string) string {
	t := strings.ToLower(typeString)
	switch {
	// --- cache (before datastore: elasticache/redis/memcached) ------------
	case containsAny(t,
		"elasticache", "elasti_cache", "memorydb", "dax",
		"aws_dax_", "::elasticache::", "::dax::", "::memorydb::",
		"elasticache.", ".dax.", "memorydb.",
		"microsoft.cache/", "/rediscache", "rediscache",
		"google_redis_", "redis.", "memcache"):
		return IaCCategoryCache

	// --- secret -----------------------------------------------------------
	case containsAny(t,
		"secretsmanager", "secrets_manager", "::secretsmanager::",
		"secretsmanager.", "aws_ssm_parameter",
		"microsoft.keyvault/", "keyvault", "key_vault",
		"google_secret_manager", "secretmanager.", ".secret"):
		return IaCCategorySecret

	// --- stream (kinesis / event hubs / pubsub streams) -------------------
	case containsAny(t,
		"kinesis", "::kinesis::", "kinesis.",
		"aws_kinesis", "msk", "managedstreaming", "::msk::",
		"microsoft.eventhub/", "eventhub",
		"google_pubsub_lite", "kafka"):
		return IaCCategoryStream

	// --- topic (pub/sub fan-out) ------------------------------------------
	case containsAny(t,
		"aws_sns_topic", "::sns::", "sns.topic", "sns_topic",
		".sns.", "snstopic",
		"microsoft.eventgrid/", "eventgrid",
		"/topics", "servicebustopic",
		"google_pubsub_topic", "pubsub.topic"):
		return IaCCategoryTopic

	// --- queue ------------------------------------------------------------
	case containsAny(t,
		"aws_sqs_queue", "::sqs::", "sqs.queue", "sqs_queue", ".sqs.", "sqsqueue",
		"aws_mq_", "::mq::", "amazonmq",
		"microsoft.servicebus/", "servicebus", "/queues", "storagequeue",
		"google_cloud_tasks", "cloudtasks",
		"::events::eventbus", "eventbus", "google_pubsub_subscription"):
		return IaCCategoryQueue

	// --- function (serverless) --------------------------------------------
	case containsAny(t,
		"aws_lambda_function", "::lambda::function", "::serverless::function",
		"lambda.function", "lambda_function", ".lambda.", "lambdafunction",
		"cfnfunction",
		"microsoft.web/sites/functions", "functionapp", "azurerm_function_app",
		"google_cloudfunctions", "cloudfunction"):
		return IaCCategoryFunction

	// --- datastore (databases + tables; tested before generic storage) ----
	case containsAny(t,
		"aws_db_instance", "aws_rds", "aws_db_cluster", "::rds::",
		"aws_dynamodb", "::dynamodb::", "dynamodb.", "dynamodbtable",
		"aws_redshift", "::redshift::", "redshift.",
		"aws_docdb", "::docdb::", "docdb.",
		"aws_neptune", "::neptune::", "neptune.",
		"aws_timestream", "::timestream::", "timestream.",
		"aws_qldb", "::qldb::", "aws_keyspaces", "::cassandra::",
		"rds.", ".rds.", "dbinstance", "dbcluster", "database",
		"microsoft.sql/", "microsoft.documentdb/", "cosmosdb", "cosmos_db",
		"microsoft.dbforpostgresql/", "microsoft.dbformysql/",
		"microsoft.dbformariadb/",
		"google_sql_", "google_spanner", "google_bigtable", "google_firestore",
		"sql.", "spanner.", "bigtable.", "firestore.",
		"rds.databaseinstance", "rds.databasecluster",
		"sql_database", "cosmosdb_account"):
		return IaCCategoryDatastore

	// --- storage (object/file/block; after datastore so DB tables win) ----
	case containsAny(t,
		"aws_s3_bucket", "::s3::bucket", "s3.bucket", "s3_bucket", ".s3.", "s3bucket",
		"aws_efs", "::efs::", "efs.", "aws_fsx", "::fsx::",
		"aws_ebs_volume", "::ec2::volume",
		"microsoft.storage/", "storageaccount", "storage_account",
		"google_storage_bucket", "storage.bucket",
		"azurerm_storage", "blobcontainer", "/blobservices"):
		return IaCCategoryStorage

	// --- network ----------------------------------------------------------
	case containsAny(t,
		"aws_vpc", "::ec2::vpc", "aws_subnet", "::ec2::subnet",
		"aws_security_group", "::ec2::securitygroup",
		"aws_route", "aws_internet_gateway", "aws_nat_gateway",
		"aws_lb", "aws_alb", "aws_elb", "::elasticloadbalancing",
		"aws_route53", "::route53::", "aws_api_gateway", "::apigateway",
		"ec2.vpc", "ec2.subnet", "ec2.securitygroup", ".vpc.", "vpc.",
		"microsoft.network/", "google_compute_network",
		"google_compute_subnetwork", "google_compute_firewall",
		"loadbalancer", "load_balancer"):
		return IaCCategoryNetwork

	// --- compute (catch-all for VMs/containers/clusters; last) ------------
	case containsAny(t,
		"aws_instance", "::ec2::instance", "ec2.instance",
		"aws_ecs", "::ecs::", "ecs.", "aws_eks", "::eks::", "eks.",
		"aws_autoscaling", "aws_batch", "::batch::",
		"microsoft.compute/", "microsoft.containerservice/", "microsoft.app/",
		"microsoft.web/sites", "appservice",
		"google_compute_instance", "google_container_cluster",
		"compute.instance", "fargate", "virtualmachine"):
		return IaCCategoryCompute

	default:
		return IaCCategoryOther
	}
}

// IaCKindForCategory maps a resource_category to the semantic SCOPE.* entity
// Kind that CloudFormation uses, so the CFN Kind and the property are derived
// from one source and can never drift. Categories without a dedicated CFN Kind
// fall back to SCOPE.InfraResource (the shared IaC-resource class). This keeps
// the historical CFN Kinds (Datastore / Queue / ServerlessFunction) intact while
// guaranteeing they agree with IaCResourceCategory.
func IaCKindForCategory(category string) string {
	switch category {
	case IaCCategoryDatastore, IaCCategoryStorage, IaCCategoryCache:
		return "SCOPE.Datastore"
	case IaCCategoryQueue, IaCCategoryTopic, IaCCategoryStream:
		return "SCOPE.Queue"
	case IaCCategoryFunction:
		return "SCOPE.ServerlessFunction"
	default:
		return "SCOPE.InfraResource"
	}
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
