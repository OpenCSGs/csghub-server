package types

import "opencsg.com/csghub-server/runner/utils"

type Validator func(value string) bool

const (
	// key name configmap and the value is endpoint of hub server runner report event
	KeyHubServerWebhookEndpoint = "STARHUB_SERVER_RUNNER_WEBHOOK_ENDPOINT"
	// key name of configmap and the value is endpoint runner exposed for hub server invoke
	KeyRunnerExposedEndpont = "STARHUB_SERVER_RUNNER_PUBLIC_DOMAIN"
	KeyRunnerClusterRegion  = "STARHUB_SERVER_CLUSTER_REGION"
	// application endpoint of cluster with remote runner
	KeyApplicationEndpoint = "STARHUB_SERVER_RUNNER_APPLICATION_ENDPOINT"
	// key name of configmap for storage class of PVC
	KeyStorageClass = "STARHUB_SERVER_RUNNER_STORAGE_CLASS"
)

var SubscribeKeyWithEventPush = map[string]Validator{
	KeyRunnerExposedEndpont: utils.ValidUrl,
}
