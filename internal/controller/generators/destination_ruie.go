package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent/api/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	apiv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
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

	if len(svc.CommitHashes) >= 1 { // +kubebuilder 검증 조건 충족
		for _, hash := range svc.CommitHashes {
			dr.Spec.Subsets = append(dr.Spec.Subsets, &apiv1beta1.Subset{
				Name:   hash,
				Labels: map[string]string{"version": hash},
			})
		}
	}

	baseType := svc.Type

	// Configure traffic policy for StickyCanary
	if baseType == meshmanagerv1.StickyCanaryType {
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

		// Add outlier detection if configured
		if svc.OutlierDetection != nil {
			addOutlierDetection(dr, svc.OutlierDetection)
		}
	}

	// For dependent services, add basic traffic policy if not already configured
	isDependent := len(svc.Dependencies) > 0
	if isDependent && dr.Spec.TrafficPolicy == nil {
		dr.Spec.TrafficPolicy = &apiv1beta1.TrafficPolicy{}
	}

	return dr
}

func addOutlierDetection(dr *istiov1beta1.DestinationRule, config *meshmanagerv1.OutlierDetection) {
	intervalDuration, _ := time.ParseDuration(config.Interval)

	if dr.Spec.TrafficPolicy == nil {
		dr.Spec.TrafficPolicy = &apiv1beta1.TrafficPolicy{}
	}

	dr.Spec.TrafficPolicy.OutlierDetection = &apiv1beta1.OutlierDetection{
		ConsecutiveErrors:  int32(config.Consecutive5xxErrors),
		Interval:           durationpb.New(intervalDuration),
		BaseEjectionTime:   durationpb.New(30 * time.Second),
		MaxEjectionPercent: 100,
		MinHealthPercent:   50,
	}
}
