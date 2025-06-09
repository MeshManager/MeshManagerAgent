package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent/api/v1"
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

	baseType := svc.Type
	isDependent := len(svc.Dependencies) > 0

	switch {
	case baseType == meshmanagerv1.CanaryType && isDependent:
		vs.Spec.Http = generateCanaryDependentRoutes(svc)
	case baseType == meshmanagerv1.StickyCanaryType && isDependent:
		vs.Spec.Http = generateStickyCanaryDependentRoutes(svc)
	case baseType == meshmanagerv1.StandardType && isDependent:
		vs.Spec.Http = generateDependentRoutes(svc)
	case baseType == meshmanagerv1.CanaryType:
		vs.Spec.Http = generateCanaryRoutes(svc)
	case baseType == meshmanagerv1.StickyCanaryType:
		vs.Spec.Http = generateStickyCanaryRoutes(svc)
	case baseType == meshmanagerv1.StandardType:
		vs.Spec.Http = generateStandardRoutes(svc)
	default:
		vs.Spec.Http = generateStandardRoutes(svc)
	}

	return vs
}

func generateCanaryRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	var routes []*apiv1beta1.HTTPRoute

	for _, hash := range svc.CommitHashes {
		route := &apiv1beta1.HTTPRoute{
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
		}
		routes = append(routes, route)
	}

	return routes
}

func generateStickyCanaryRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	return generateCanaryRoutes(svc)
}

func generateCanaryDependentRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	var routes []*apiv1beta1.HTTPRoute

	for _, hash := range svc.CommitHashes {
		// Create headers map that includes both canary version and dependency headers
		headers := map[string]*apiv1beta1.StringMatch{
			"x-canary-version": {
				MatchType: &apiv1beta1.StringMatch_Exact{
					Exact: hash,
				},
			},
		}

		// Add dependency headers
		for _, dep := range svc.Dependencies {
			headerName := fmt.Sprintf("x-%s-version", dep.Name)
			if len(dep.CommitHashes) > 0 {
				headers[headerName] = &apiv1beta1.StringMatch{
					MatchType: &apiv1beta1.StringMatch_Exact{
						Exact: dep.CommitHashes[0],
					},
				}
			}
		}

		route := &apiv1beta1.HTTPRoute{
			Match: []*apiv1beta1.HTTPMatchRequest{
				{
					Headers: headers,
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
		}
		routes = append(routes, route)
	}

	return routes
}

func generateStickyCanaryDependentRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	return generateCanaryDependentRoutes(svc) // Same structure as Canary+Dependent
}

func generateDependentRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	headers := make(map[string]*apiv1beta1.StringMatch)

	for _, dep := range svc.Dependencies {
		headerName := fmt.Sprintf("x-%s-version", dep.Name)
		if len(dep.CommitHashes) > 0 {
			headers[headerName] = &apiv1beta1.StringMatch{
				MatchType: &apiv1beta1.StringMatch_Exact{
					Exact: dep.CommitHashes[0],
				},
			}
		}
	}

	route := &apiv1beta1.HTTPRoute{
		Match: []*apiv1beta1.HTTPMatchRequest{
			{
				Headers: headers,
			},
		},
		Route: []*apiv1beta1.HTTPRouteDestination{
			{
				Destination: &apiv1beta1.Destination{
					Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
				},
			},
		},
	}

	return []*apiv1beta1.HTTPRoute{route}
}

func generateStandardRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	route := &apiv1beta1.HTTPRoute{
		Route: []*apiv1beta1.HTTPRouteDestination{
			{
				Destination: &apiv1beta1.Destination{
					Host: fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
				},
			},
		},
	}

	return []*apiv1beta1.HTTPRoute{route}
}
