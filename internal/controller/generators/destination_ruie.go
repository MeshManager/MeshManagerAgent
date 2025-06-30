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

	// 서브셋을 추적하기 위한 맵 (중복 방지)
	subsetMap := make(map[string]struct{})

	// 1. CommitHashes에서 서브셋 추가
	for _, hash := range svc.CommitHashes {
		if _, exists := subsetMap[hash]; !exists {
			dr.Spec.Subsets = append(dr.Spec.Subsets, &apiv1beta1.Subset{
				Name:   hash,
				Labels: map[string]string{"version": hash},
			})
			subsetMap[hash] = struct{}{}
		}
	}

	// 2. DarknessReleases에서 서브셋 추가
	for _, drRelease := range svc.DarknessReleases {
		hash := drRelease.CommitHash
		if _, exists := subsetMap[hash]; !exists {
			dr.Spec.Subsets = append(dr.Spec.Subsets, &apiv1beta1.Subset{
				Name:   hash,
				Labels: map[string]string{"version": hash},
			})
			subsetMap[hash] = struct{}{}
		}
	}

	baseType := svc.Type

	// StickyCanaryType 트래픽 정책 설정
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

		if svc.OutlierDetection != nil {
			addOutlierDetection(dr, svc.OutlierDetection)
		}
	}

	// 종속 서비스 처리
	if len(svc.Dependencies) > 0 && dr.Spec.TrafficPolicy == nil {
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
