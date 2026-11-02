package common

import v1 "k8s.io/api/core/v1"

const QuotaRequest v1.ResourceName = "requests."
const DefaultResourceName = "nvidia.com/gpu"
const MIGResourcePrefix string = "nvidia.com/mig-"

