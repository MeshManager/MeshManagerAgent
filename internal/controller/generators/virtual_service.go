package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent.git/api/v1"
	apiv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateVirtualService(svc meshmanagerv1.ServiceConfig) *istiov1beta1.VirtualService {
	vs := &istiov1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
		},
		Spec: apiv1beta1.VirtualService{
			Hosts: []string{fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)},
		},
	}

	for _, hash := range svc.CommitHashes {
		vs.Spec.Http = append(vs.Spec.Http, &apiv1beta1.HTTPRoute{
			Match: []*apiv1beta1.HTTPMatchRequest{
				{
					Headers: map[string]*apiv1beta1.StringMatch{
						"x-canary-version": {
							MatchType: &apiv1beta1.StringMatch_Exact{
								Exact: hash,
							},
						},
					},
				},
			},
			Route: []*apiv1beta1.HTTPRouteDestination{
				{
					Destination: &apiv1beta1.Destination{
						Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
						Subset: hash,
					},
				},
			},
		})
	}

	return vs
}
