SET statement_timeout = 0;

--bun:split

ALTER TABLE cluster_nodes DROP COLUMN IF EXISTS xpu_type;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS cluster_node;

--bun:split

ALTER TABLE deploys DROP COLUMN IF EXISTS queue_name;

--bun:split

ALTER TABLE knative_services DROP COLUMN IF EXISTS cluster_node;

--bun:split

ALTER TABLE knative_services DROP COLUMN IF EXISTS queue_name;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS cluster_node;

--bun:split

ALTER TABLE argo_workflows DROP COLUMN IF EXISTS queue_name;

--bun:split

DROP INDEX IF EXISTS idx_deploys_cluster_node_status;

--bun:split

DROP INDEX IF EXISTS idx_deploys_sku;

--bun:split

DROP INDEX IF EXISTS idx_argo_workflows_cluster_node_status;

--bun:split

DROP INDEX IF EXISTS idx_knative_services_cluster_node_status;

--bun:split

DROP INDEX IF EXISTS idx_knative_services_sku;
