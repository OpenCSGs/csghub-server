package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/component"
)

type ClusterHandler struct {
	k8sNameSpace       string
	modelDockerRegBase string
	clusterComponent   component.ClusterComponent
}

func NewClusterHandler(config *config.Config, clusterPool cluster.Pool) (*ClusterHandler, error) {
	clusterComponent := component.NewClusterComponent(config, clusterPool)
	return &ClusterHandler{
		k8sNameSpace:       config.Cluster.SpaceNamespace,
		clusterComponent:   clusterComponent,
		modelDockerRegBase: config.Model.DockerRegBase,
	}, nil
}

func (s *ClusterHandler) GetClusterInfoByID(c *gin.Context) {
	clusterId := c.Params.ByName("id")
	cInfo, _ := s.clusterComponent.ByClusterID(c.Request.Context(), clusterId)
	clusterInfo := types.ClusterResponse{}
	clusterInfo.Region = cInfo.Region
	clusterInfo.Zone = cInfo.Zone
	clusterInfo.Provider = cInfo.Provider
	clusterInfo.ClusterID = cInfo.ClusterID
	clusterInfo.StorageClass = cInfo.StorageClass
	availabilityStatus, resourceAvaliable, err := s.clusterComponent.GetResourceByID(c.Request.Context(), clusterId)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fail to get cluster", slog.Any("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	clusterInfo.Nodes = resourceAvaliable
	clusterInfo.ResourceStatus = availabilityStatus
	c.JSON(http.StatusOK, clusterInfo)
}
