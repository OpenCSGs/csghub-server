SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_nodes ADD COLUMN IF NOT EXISTS xpu_type VARCHAR(255);

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS cluster_node VARCHAR(255);

--bun:split

ALTER TABLE deploys ADD COLUMN IF NOT EXISTS queue_name VARCHAR(255);

--bun:split

ALTER TABLE knative_services ADD COLUMN IF NOT EXISTS cluster_node VARCHAR(255);

--bun:split

ALTER TABLE knative_services ADD COLUMN IF NOT EXISTS queue_name VARCHAR(255);

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS cluster_node VARCHAR(255);

--bun:split

ALTER TABLE argo_workflows ADD COLUMN IF NOT EXISTS queue_name VARCHAR(255);

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_cluster_node_status ON deploys (cluster_id, cluster_node, status);

--bun:split

CREATE INDEX IF NOT EXISTS idx_deploys_sku ON deploys (sku);

--bun:split

CREATE INDEX IF NOT EXISTS idx_argo_workflows_cluster_node_status ON argo_workflows (cluster_id, cluster_node, status);

--bun:split

CREATE INDEX IF NOT EXISTS idx_knative_services_cluster_node_status ON knative_services (cluster_id, cluster_node, status);

--bun:split

CREATE INDEX IF NOT EXISTS idx_knative_services_sku ON knative_services (deploy_sku);
