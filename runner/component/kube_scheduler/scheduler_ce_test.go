//go:build !saas && !ee

package kube_scheduler

import (
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/common/types"
)

func TestNewApplier_CE(t *testing.T) {
	tests := []struct {
		name     string
		config   *types.Scheduler
		wantType Applier
	}{
		{
			name:     "nil config",
			config:   nil,
			wantType: &DefaultOpApplier{},
		},
		{
			name:     "empty config",
			config:   &types.Scheduler{},
			wantType: &DefaultOpApplier{},
		},
		{
			name: "volcano config (ignored in CE)",
			config: &types.Scheduler{
				Volcano: &types.VolcanoConfig{
					SchedulerName: "volcano",
				},
			},
			wantType: &DefaultOpApplier{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewApplier(tt.config)
			assert.IsType(t, tt.wantType, got)
		})
	}
}

func TestDefaultOpApplier_ApplyToArgo(t *testing.T) {
	applier := &DefaultOpApplier{}
	template := &v1alpha1.Template{
		Name: "test-template",
		Container: &corev1.Container{
			Name:  "main",
			Image: "nginx",
		},
		InitContainers: []v1alpha1.UserContainer{
			{
				Container: corev1.Container{
					Name:  "init",
					Image: "busybox",
				},
			},
		},
	}

	err := applier.ApplyToArgo(template)
	assert.NoError(t, err)

	assert.Nil(t, template.Resource)
	assert.NotNil(t, template.Container)
	assert.Equal(t, "main", template.Container.Name)
	assert.Equal(t, "nginx", template.Container.Image)

	assert.Len(t, template.InitContainers, 1)
	assert.Equal(t, "init", template.InitContainers[0].Container.Name)
}
