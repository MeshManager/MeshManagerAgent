package generators

import (
	"fmt"
	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent/api/v1"
	"google.golang.org/protobuf/types/known/structpb"
	apiv1beta1 "istio.io/api/networking/v1alpha3"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func GenerateEnvoyFilter(svc meshmanagerv1.ServiceConfig, istioRoute *meshmanagerv1.IstioRoute) *istiov1beta1.EnvoyFilter {
	return &istiov1beta1.EnvoyFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-filter", svc.Name),
			Namespace: "istio-system",
			Labels: map[string]string{
				"managed-by":           "istioroute-controller",
				"istioroute-name":      istioRoute.Name,
				"istioroute-namespace": istioRoute.Namespace,
				"istioroute-type":      "envoy-filter",
			},
			Annotations: map[string]string{
				"istioroute-controller/managed":         "true",
				"istioroute-controller/owner-name":      istioRoute.Name,
				"istioroute-controller/owner-namespace": istioRoute.Namespace,
				"istioroute-controller/owner-uid":       string(istioRoute.UID),
			},
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
	baseType := svc.Type
	isDependent := len(svc.Dependencies) > 0

	var luaScript string

	switch {
	case baseType == meshmanagerv1.CanaryType && isDependent:
		luaScript = buildCanaryDependentLuaScript(svc)
	case baseType == meshmanagerv1.StickyCanaryType && isDependent:
		luaScript = buildStickyCanaryDependentLuaScript(svc)
	case baseType == meshmanagerv1.CanaryType:
		luaScript = buildCanaryLuaScript(svc)
	case baseType == meshmanagerv1.StickyCanaryType:
		luaScript = buildStickyCanaryLuaScript(svc)
	}

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

func buildCanaryLuaScript(svc meshmanagerv1.ServiceConfig) string {
	if len(svc.CommitHashes) < 2 {
		return ""
	}

	return fmt.Sprintf(`
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
		end
	end
end`, svc.Name, svc.Namespace, svc.Ratio, svc.CommitHashes[0], svc.CommitHashes[1])
}

func buildStickyCanaryLuaScript(svc meshmanagerv1.ServiceConfig) string {
	if len(svc.CommitHashes) < 2 {
		return ""
	}

	return fmt.Sprintf(`
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
			headers:add("x-session-id", tostring(math.floor(hash)))
		end
	end
end`, svc.Name, svc.Namespace, svc.Ratio, svc.CommitHashes[0], svc.CommitHashes[1])
}

func buildCanaryDependentLuaScript(svc meshmanagerv1.ServiceConfig) string {
	if len(svc.CommitHashes) < 2 {
		return ""
	}

	// Build dependency header additions
	var depHeaderLines []string
	for _, dep := range svc.Dependencies {
		if len(dep.CommitHashes) > 0 {
			line := fmt.Sprintf(`			headers:add("x-%s-version", "%s")`, dep.Name, dep.CommitHashes[0])
			depHeaderLines = append(depHeaderLines, line)
		}
	}
	depHeaderCode := strings.Join(depHeaderLines, "\n")

	return fmt.Sprintf(`
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
%s
		end
	end
end`, svc.Name, svc.Namespace, svc.Ratio, svc.CommitHashes[0], svc.CommitHashes[1], depHeaderCode)
}

func buildStickyCanaryDependentLuaScript(svc meshmanagerv1.ServiceConfig) string {
	if len(svc.CommitHashes) < 2 {
		return ""
	}

	// Build dependency header additions
	var depHeaderLines []string
	for _, dep := range svc.Dependencies {
		if len(dep.CommitHashes) > 0 {
			line := fmt.Sprintf(`			headers:add("x-%s-version", "%s")`, dep.Name, dep.CommitHashes[0])
			depHeaderLines = append(depHeaderLines, line)
		}
	}
	depHeaderCode := strings.Join(depHeaderLines, "\n")

	return fmt.Sprintf(`
function envoy_on_request(request_handle)
	local headers = request_handle:headers()
  	local path = headers:get(":path")
  
  	if string.find(path, "^/%s") then
		local jwt = headers:get("jwt")
		if jwt then
			local hash = 0
			for i = 1, #jwt do
				hash = (hash * 31 + jwt:byte(i)) %% 100
			end
			
			headers:add("x-canary-version", hash < %d and "%s" or "%s")
			headers:add("x-session-id", tostring(math.floor(hash)))
%s
		end
	end
end`, svc.Name, svc.Ratio, svc.CommitHashes[0], svc.CommitHashes[1], depHeaderCode)
}
