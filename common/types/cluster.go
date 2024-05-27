package types

type Cluster struct {
	ClusterID     string `json:"cluster_id"`
	ClusterConfig string `json:"cluster_config"`
	Region        string `json:"region"`
}
