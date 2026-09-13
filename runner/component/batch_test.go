//go:build saas || ee

package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"opencsg.com/csghub-server/builder/deploy/common"
)

// ksvcCodeFromServiceAndPods matches pods by the Knative-injected label
// serving.knative.dev/service=<svcName>. The Knative Route Service has an
// empty spec.selector, and an empty selector matches ALL pods in the
// namespace (Everything()), so readiness of unrelated services used to leak
// into the count. These cases pin that behavior.

func TestKsvcCodeFromServiceAndPods(t *testing.T) {
	newPod := func(name, svcName string, ready bool, terminating bool) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{KeyServiceLabel: svcName},
			},
		}
		if ready {
			p.Status.Conditions = []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			}
		} else {
			p.Status.Conditions = []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			}
		}
		if terminating {
			now := metav1.Now()
			p.DeletionTimestamp = &now
		}
		return p
	}

	// Knative Route Service (ExternalName) carries no spec.selector,
	// mirroring what BatchKsvcStatus feeds in.
	svc := func(name string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}
	}

	tests := []struct {
		name string
		svc  *corev1.Service
		pods []*corev1.Pod
		want int
	}{
		{
			name: "ready pods of own service only -> Running",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-a1", "svc-a", true, false),
			},
			want: common.Running,
		},
		{
			name: "unrelated unready pod must not pollute own count -> Running",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-a1", "svc-a", true, false),
				newPod("pod-b1", "svc-b", false, false),
			},
			want: common.Running,
		},
		{
			name: "no matching pod -> Startup",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-b1", "svc-b", true, false),
			},
			want: common.Startup,
		},
		{
			name: "nil pod list -> Startup",
			svc:  svc("svc-a"),
			pods: nil,
			want: common.Startup,
		},
		{
			name: "own pods all unready -> Startup",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-a1", "svc-a", false, false),
				newPod("pod-a2", "svc-a", false, false),
			},
			want: common.Startup,
		},
		{
			name: "terminating ready pod is ignored -> Startup",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-a1", "svc-a", true, true),
			},
			want: common.Startup,
		},
		{
			name: "terminating pod ignored while live ready pod counts -> Running",
			svc:  svc("svc-a"),
			pods: []*corev1.Pod{
				newPod("pod-a1", "svc-a", true, true),
				newPod("pod-a2", "svc-a", true, false),
			},
			want: common.Running,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var podList *corev1.PodList
			if tt.pods != nil {
				podList = &corev1.PodList{}
				for _, p := range tt.pods {
					podList.Items = append(podList.Items, *p)
				}
			}
			got := ksvcCodeFromServiceAndPods(tt.svc, podList)
			require.Equal(t, tt.want, got)
		})
	}
}
