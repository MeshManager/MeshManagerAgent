package metrics_service

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func ExtractServiceInfo(list *corev1.ServiceList) []map[string]interface{} {
	var services []map[string]interface{}
	for _, svc := range list.Items {
		services = append(services, map[string]interface{}{
			"name":      svc.Name,
			"type":      string(svc.Spec.Type),
			"clusterIP": svc.Spec.ClusterIP,
			"selector":  svc.Spec.Selector,
		})
	}
	return services
}

func ExtractDeploymentInfo(list *appsv1.DeploymentList) []map[string]interface{} {
	var deployments []map[string]interface{}
	for _, deploy := range list.Items {

		containers := make([]map[string]string, 0)
		for _, c := range deploy.Spec.Template.Spec.Containers {
			containers = append(containers, map[string]string{
				"name":  c.Name,
				"image": c.Image,
			})
		}

		deployments = append(deployments, map[string]interface{}{
			"name":       deploy.Name,
			"replicas":   *deploy.Spec.Replicas,
			"containers": containers,
			"podLabels":  deploy.Spec.Template.Labels,
		})
	}
	return deployments
}
