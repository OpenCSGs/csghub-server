package inference

type LlmModelInfo struct {
	URL    map[string]string              `json:"url"`
	Status map[string]LlmModelInfo_Status `json:"status"`
}

type LlmModelInfo_Status struct {
	/*example:
	* {
	*	"OpenCSG--opencsg-CodeLlama-7b-v0.1": "HEALTHY",
	*	"RouterDeployment": "HEALTHY"
	*	}
	 */
	DeploymentsStatus map[string]string `json:"deployments_status"`
	// example: RUNNING
	ApplicationStatus string `json:"application_status"`
}
