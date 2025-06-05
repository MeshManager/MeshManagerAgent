package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent.git/api/v1"
	"google.golang.org/protobuf/types/known/structpb"
	apiv1beta1 "istio.io/api/networking/v1alpha3"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateEnvoyFilter(svc meshmanagerv1.ServiceConfig) *istiov1beta1.EnvoyFilter {
	return &istiov1beta1.EnvoyFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-envoyfilter", svc.Name),
			Namespace: "istio-system",
		},
		Spec: apiv1beta1.EnvoyFilter{
			WorkloadSelector: &apiv1beta1.WorkloadSelector{
				Labels: map[string]string{"istio": "ingressgateway"},
			},
			ConfigPatches: []*apiv1beta1.EnvoyFilter_EnvoyConfigObjectPatch{
				{
					ApplyTo: apiv1beta1.EnvoyFilter_HTTP_FILTER,
					Match: &apiv1beta1.EnvoyFilter_EnvoyConfigObjectMatch{
						Context: apiv1beta1.EnvoyFilter_GATEWAY,
						ObjectTypes: &apiv1beta1.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
							Listener: &apiv1beta1.EnvoyFilter_ListenerMatch{
								FilterChain: &apiv1beta1.EnvoyFilter_ListenerMatch_FilterChainMatch{
									Filter: &apiv1beta1.EnvoyFilter_ListenerMatch_FilterMatch{
										Name: "envoy.filters.network.http_connection_manager",
									},
								},
							},
						},
					},
					Patch: &apiv1beta1.EnvoyFilter_Patch{
						Operation: apiv1beta1.EnvoyFilter_Patch_INSERT_BEFORE,
						Value:     buildLuaFilterConfig(svc),
					},
				},
			},
		},
	}
}

func buildLuaFilterConfig(svc meshmanagerv1.ServiceConfig) *structpb.Struct {
	luaScript := fmt.Sprintf(`
		function envoy_on_request(request_handle)
			local headers = request_handle:headers()
			local host = headers:get(":authority")
			
			if host == "%s.%s.svc.cluster.local" then
				local jwt = headers:get("jwt")
				if jwt then
					local hash = 0
					for i = 1, #jwt do
						hash = (hash * 31 + jwt:byte(i)) %% 100
					end
					
					headers:add("x-canary-version", hash < %d and "%s" or "%s")
					headers:add("x-session-id", string.format("%%s-%%d", host, hash))
				end
			end
		end
	`, svc.Name, svc.Namespace, svc.Ratio, svc.CommitHashes[0], svc.CommitHashes[1])

	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name": structpb.NewStringValue("envoy.lua"),
			"typed_config": structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"@type":       structpb.NewStringValue("type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua"),
					"inline_code": structpb.NewStringValue(luaScript),
				},
			}),
		},
	}
}
