package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/common/types"
)

func TestMergeNodeAffinity(t *testing.T) {
	// Test case 1: Merge two nil affinities
	t.Run("MergeNil", func(t *testing.T) {
		result := MergeNodeAffinity(nil, nil)
		assert.Nil(t, result)
	})

	// Test case 2: Merge one nil and one non-nil
	t.Run("MergeOneNil", func(t *testing.T) {
		a := &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpIn, Values: []string{"bar"}}}},
				},
			},
		}
		result := MergeNodeAffinity(a, nil)
		assert.Equal(t, a.RequiredDuringSchedulingIgnoredDuringExecution, result.RequiredDuringSchedulingIgnoredDuringExecution)
	})

	// Test case 3: Merge two affinities
	t.Run("MergeTwo", func(t *testing.T) {
		a := &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "a", Operator: corev1.NodeSelectorOpIn, Values: []string{"1"}}}},
				},
			},
		}
		b := &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "b", Operator: corev1.NodeSelectorOpIn, Values: []string{"2"}}}},
				},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{Weight: 1, Preference: corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "p", Operator: corev1.NodeSelectorOpIn, Values: []string{"3"}}}}},
			},
		}

		result := MergeNodeAffinity(a, b)
		assert.NotNil(t, result)
		assert.Len(t, result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 1)
		assert.Len(t, result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, 2)
		assert.Len(t, result.PreferredDuringSchedulingIgnoredDuringExecution, 1)
	})

	// Test case 4: Merge list
	t.Run("MergeList", func(t *testing.T) {
		a := &corev1.NodeAffinity{}
		b := &corev1.NodeAffinity{}
		c := &corev1.NodeAffinity{}
		result := MergeNodeAffinity(a, b, c)
		assert.NotNil(t, result)
	})
}

func TestToCoreV1Tolerations(t *testing.T) {
	input := []types.Toleration{
		{Key: "key1", Operator: "Equal", Value: "val1", Effect: "NoSchedule"},
	}
	output := ToCoreV1Tolerations(input)
	assert.Len(t, output, 1)
	assert.Equal(t, "key1", output[0].Key)
	assert.Equal(t, corev1.TolerationOpEqual, output[0].Operator)
	assert.Equal(t, "val1", output[0].Value)
	assert.Equal(t, corev1.TaintEffectNoSchedule, output[0].Effect)
}

func TestFillAffinity(t *testing.T) {
	t.Run("NewAffinity", func(t *testing.T) {
		var target *corev1.Affinity
		source := &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpIn, Values: []string{"bar"}}}},
				},
			},
		}
		FillAffinity(&target, source)
		assert.NotNil(t, target)
		assert.NotNil(t, target.NodeAffinity)
		assert.Equal(t, source.RequiredDuringSchedulingIgnoredDuringExecution, target.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	})

	t.Run("UpdateExistingAffinity", func(t *testing.T) {
		target := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "existing", Operator: corev1.NodeSelectorOpIn, Values: []string{"val"}}}},
					},
				},
			},
		}
		source := &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "new", Operator: corev1.NodeSelectorOpIn, Values: []string{"val"}}}},
				},
			},
		}

		// Pass target.NodeAffinity to merge with it
		FillAffinity(&target, target.NodeAffinity, source)

		assert.NotNil(t, target.NodeAffinity)
		assert.Len(t, target.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 1)
		assert.Len(t, target.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, 2)
	})
}

func TestFillTolerations(t *testing.T) {
	var target []corev1.Toleration
	source := []types.Toleration{
		{Key: "key1", Operator: "Equal", Value: "val1", Effect: "NoSchedule"},
	}

	FillTolerations(&target, source)
	assert.Len(t, target, 1)

	FillTolerations(&target, source)
	assert.Len(t, target, 2)
}
