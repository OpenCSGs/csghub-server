package utils

import (
	corev1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/common/types"
)

// MergeNodeAffinity merges multiple NodeAffinity objects into one.
// It prioritizes later arguments.
func MergeNodeAffinity(affinities ...*corev1.NodeAffinity) *corev1.NodeAffinity {
	var result *corev1.NodeAffinity
	for _, affinity := range affinities {
		if affinity == nil {
			continue
		}
		if result == nil {
			result = &corev1.NodeAffinity{}
		}
		// Merge RequiredDuringSchedulingIgnoredDuringExecution
		if affinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			if result.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				result.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
					NodeSelectorTerms: cloneNodeSelectorTerms(affinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms),
				}
			} else {
				result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = andNodeSelectorTerms(
					result.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
					affinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				)
			}
		}
		// Merge PreferredDuringSchedulingIgnoredDuringExecution
		if len(affinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
			result.PreferredDuringSchedulingIgnoredDuringExecution = append(
				result.PreferredDuringSchedulingIgnoredDuringExecution,
				affinity.PreferredDuringSchedulingIgnoredDuringExecution...,
			)
		}
	}
	return result
}

func andNodeSelectorTerms(existing, incoming []corev1.NodeSelectorTerm) []corev1.NodeSelectorTerm {
	if len(existing) == 0 {
		return cloneNodeSelectorTerms(incoming)
	}
	if len(incoming) == 0 {
		return cloneNodeSelectorTerms(existing)
	}

	merged := make([]corev1.NodeSelectorTerm, 0, len(existing)*len(incoming))
	for _, left := range existing {
		for _, right := range incoming {
			merged = append(merged, corev1.NodeSelectorTerm{
				MatchExpressions: append(
					append([]corev1.NodeSelectorRequirement{}, left.MatchExpressions...),
					right.MatchExpressions...,
				),
				MatchFields: append(
					append([]corev1.NodeSelectorRequirement{}, left.MatchFields...),
					right.MatchFields...,
				),
			})
		}
	}

	return merged
}

func cloneNodeSelectorTerms(terms []corev1.NodeSelectorTerm) []corev1.NodeSelectorTerm {
	if len(terms) == 0 {
		return nil
	}

	cloned := make([]corev1.NodeSelectorTerm, 0, len(terms))
	for _, term := range terms {
		clonedTerm := corev1.NodeSelectorTerm{}
		if len(term.MatchExpressions) > 0 {
			clonedTerm.MatchExpressions = append([]corev1.NodeSelectorRequirement{}, term.MatchExpressions...)
		}
		if len(term.MatchFields) > 0 {
			clonedTerm.MatchFields = append([]corev1.NodeSelectorRequirement{}, term.MatchFields...)
		}
		cloned = append(cloned, clonedTerm)
	}

	return cloned
}

func ToCoreV1Tolerations(tolerations []types.Toleration) []corev1.Toleration {
	if len(tolerations) == 0 {
		return nil
	}
	var result []corev1.Toleration
	for _, t := range tolerations {
		result = append(result, corev1.Toleration{
			Key:      t.Key,
			Operator: corev1.TolerationOperator(t.Operator),
			Value:    t.Value,
			Effect:   corev1.TaintEffect(t.Effect),
		})
	}
	return result
}

// FillAffinity merges the provided node affinities and updates the target Affinity.
// It supports merging into an existing Affinity (if passed as one of the affinities) or creating a new one.
// Note: To merge with existing NodeAffinity in the target, pass (*target).NodeAffinity as one of the arguments.
func FillAffinity(target **corev1.Affinity, affinities ...*corev1.NodeAffinity) {
	merged := MergeNodeAffinity(affinities...)
	if merged != nil {
		if *target == nil {
			*target = &corev1.Affinity{}
		}
		(*target).NodeAffinity = merged
	}
}

func FillTolerations(target *[]corev1.Toleration, reqTolerations []types.Toleration) {
	if len(reqTolerations) > 0 {
		k8sTolerations := ToCoreV1Tolerations(reqTolerations)
		*target = append(*target, k8sTolerations...)
	}
}
