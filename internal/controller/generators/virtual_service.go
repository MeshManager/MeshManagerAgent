package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent/api/v1"
	apiv1beta1 "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
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

	var mainRoutes []*apiv1beta1.HTTPRoute

	baseType := svc.Type
	isDependent := len(svc.Dependencies) > 0

	switch {
	case baseType == meshmanagerv1.CanaryType && isDependent:
		mainRoutes = generateCanaryDependentRoutes(svc)
	case baseType == meshmanagerv1.StickyCanaryType && isDependent:
		mainRoutes = generateStickyCanaryDependentRoutes(svc)
	case baseType == meshmanagerv1.StandardType && isDependent:
		mainRoutes = generateDependentRoutes(svc)
	case baseType == meshmanagerv1.CanaryType:
		mainRoutes = generateCanaryRoutes(svc)
	case baseType == meshmanagerv1.StickyCanaryType:
		mainRoutes = generateStickyCanaryRoutes(svc)
	case baseType == meshmanagerv1.StandardType:
		mainRoutes = generateStandardRoutes(svc)
	default:
		mainRoutes = generateStandardRoutes(svc)
	}

	var allRoutes []*apiv1beta1.HTTPRoute
	if len(svc.DarknessReleases) > 0 {
		drRoutes := generateDarknessReleaseRoutes(svc)
		allRoutes = append(drRoutes, mainRoutes...)
	} else {
		allRoutes = mainRoutes
	}

	vs.Spec.Http = allRoutes

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
	defaultSubset := getDefaultSubset(svc)
	route := &apiv1beta1.HTTPRoute{
		Route: []*apiv1beta1.HTTPRouteDestination{
			{
				Destination: &apiv1beta1.Destination{
					Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
					Subset: defaultSubset,
				},
			},
		},
	}

	return []*apiv1beta1.HTTPRoute{route}
}

func generateDarknessReleaseRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	var routes []*apiv1beta1.HTTPRoute

	for _, dr := range svc.DarknessReleases {
		for _, ip := range dr.IPs {
			regexPattern := convertSingleIPToRegex(ip)

			route := &apiv1beta1.HTTPRoute{
				Match: []*apiv1beta1.HTTPMatchRequest{
					{
						Headers: map[string]*apiv1beta1.StringMatch{
							"x-forwarded-for": {
								MatchType: &apiv1beta1.StringMatch_Regex{
									Regex: regexPattern,
								},
							},
						},
					},
				},
				Route: []*apiv1beta1.HTTPRouteDestination{
					{
						Destination: &apiv1beta1.Destination{
							Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
							Subset: dr.CommitHash,
						},
					},
				},
			}
			routes = append(routes, route)
		}
	}
	return routes
}

// 단일 IP를 XFF 헤더 검사용 정규식으로 변환
func convertSingleIPToRegex(ip string) string {
	escapedIP := regexp.QuoteMeta(ip)
	return fmt.Sprintf(`(^|, )%s(,|$)`, escapedIP)
}

func getDefaultSubset(svc meshmanagerv1.ServiceConfig) string {
	darknessHashes := make(map[string]struct{})
	for _, dr := range svc.DarknessReleases {
		darknessHashes[dr.CommitHash] = struct{}{}
	}

	for _, ch := range svc.CommitHashes {
		if _, exists := darknessHashes[ch]; !exists {
			return ch // darkness에 없는 첫 번째 커밋 해시
		}
	}
	return svc.CommitHashes[0] // fallback
}

func GenerateIngressVirtualService(svc meshmanagerv1.ServiceConfig) *istiov1beta1.VirtualService {
	vs := &istiov1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name + "-ingress",
			Namespace: svc.Namespace,
		},
		Spec: apiv1beta1.VirtualService{
			Hosts:    []string{"*"},
			Gateways: []string{"istio-gateway"},
			Http:     generateIngressRoutes(svc),
		},
	}
	return vs
}

func generateIngressRoutes(svc meshmanagerv1.ServiceConfig) []*apiv1beta1.HTTPRoute {
	uriPrefix := fmt.Sprintf("/%s", svc.Name) // 서비스명 기반 경로 생성

	var routes []*apiv1beta1.HTTPRoute

	for _, dr := range svc.DarknessReleases {
		for _, ip := range dr.IPs {
			regexPattern := convertSingleIPToRegex(ip)
			route := &apiv1beta1.HTTPRoute{
				Match: []*apiv1beta1.HTTPMatchRequest{{
					Uri: &apiv1beta1.StringMatch{
						MatchType: &apiv1beta1.StringMatch_Prefix{Prefix: uriPrefix},
					},
					Headers: map[string]*apiv1beta1.StringMatch{
						"x-envoy-external-address": {
							MatchType: &apiv1beta1.StringMatch_Regex{
								Regex: regexPattern,
							},
						},
					},
				}},
				Rewrite: &apiv1beta1.HTTPRewrite{Uri: "/api"},
				Route: []*apiv1beta1.HTTPRouteDestination{{
					Destination: &apiv1beta1.Destination{
						Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
						Subset: dr.CommitHash,
					},
				}},
			}
			routes = append(routes, route)
		}
	}

	for _, hash := range svc.CommitHashes {
		route := &apiv1beta1.HTTPRoute{
			Match: []*apiv1beta1.HTTPMatchRequest{{
				Uri: &apiv1beta1.StringMatch{
					MatchType: &apiv1beta1.StringMatch_Prefix{Prefix: uriPrefix},
				},
				Headers: map[string]*apiv1beta1.StringMatch{
					"x-canary-version": {
						MatchType: &apiv1beta1.StringMatch_Exact{Exact: hash},
					},
				},
			}},
			Rewrite: &apiv1beta1.HTTPRewrite{Uri: "/api"},
			Route: []*apiv1beta1.HTTPRouteDestination{{
				Destination: &apiv1beta1.Destination{
					Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
					Subset: hash,
				},
			}},
		}
		routes = append(routes, route)
	}

	defaultRoute := &apiv1beta1.HTTPRoute{
		Match: []*apiv1beta1.HTTPMatchRequest{{
			Uri: &apiv1beta1.StringMatch{
				MatchType: &apiv1beta1.StringMatch_Prefix{Prefix: uriPrefix},
			},
		}},
		Rewrite: &apiv1beta1.HTTPRewrite{Uri: "/api"},
		Route: []*apiv1beta1.HTTPRouteDestination{{
			Destination: &apiv1beta1.Destination{
				Host:   fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
				Subset: getDefaultSubset(svc),
			},
		}},
	}
	routes = append(routes, defaultRoute)

	return routes
}
