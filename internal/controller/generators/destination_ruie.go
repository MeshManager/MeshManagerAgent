package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent.git/api/v1"
	apiv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateDestinationRule(svc meshmanagerv1.ServiceConfig) *istiov1beta1.DestinationRule {
	dr := &istiov1beta1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: apiv1beta1.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
		},
	}

	// Add subsets
	for _, hash := range svc.CommitHashes {
		dr.Spec.Subsets = append(dr.Spec.Subsets, &apiv1beta1.Subset{
			Name:   hash,
			Labels: map[string]string{"commit": hash},
		})
	}

	// Configure traffic policy
	if svc.Type == meshmanagerv1.StickyCanaryType {
		dr.Spec.TrafficPolicy = &apiv1beta1.TrafficPolicy{
			LoadBalancer: &apiv1beta1.LoadBalancerSettings{
				LbPolicy: &apiv1beta1.LoadBalancerSettings_ConsistentHash{
					ConsistentHash: &apiv1beta1.LoadBalancerSettings_ConsistentHashLB{
						HashKey: &apiv1beta1.LoadBalancerSettings_ConsistentHashLB_HttpHeaderName{
							HttpHeaderName: "x-session-id",
						},
					},
				},
			},
		}
	}

	// Add outlier detection if configured
	if svc.OutlierDetection != nil {
		//TODO outlier Detection 제작
	}

	return dr
}
