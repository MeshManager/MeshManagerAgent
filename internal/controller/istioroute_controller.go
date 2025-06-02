/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"github.com/MeshManager/MeshManagerAgent.git/internal/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent.git/api/v1"
)

// IstioRouteReconciler reconciles a IstioRoute object
type IstioRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mesh-manager.meshmanager.com,resources=istioroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh-manager.meshmanager.com,resources=istioroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mesh-manager.meshmanager.com,resources=istioroutes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IstioRoute object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *IstioRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	err := r.sendMetric(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1.IstioRoute{}).
		Complete(r)
}

func (r *IstioRouteReconciler) sendMetric(ctx context.Context) error {
	// 1. istio-injection=enabled인 네임스페이스 조회
	nsList := &corev1.NamespaceList{}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{"istio-injection": "enabled"},
	}
	selector, _ := metav1.LabelSelectorAsSelector(&labelSelector)
	err := r.List(ctx, nsList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}

	for _, ns := range nsList.Items {
		// 서비스 조회
		svcList := &corev1.ServiceList{}
		err := r.List(ctx, svcList, client.InNamespace(ns.Name))
		if err != nil {
			continue
		}

		// 디플로이먼트 조회
		deployList := &appsv1.DeploymentList{}
		err = r.List(ctx, deployList, client.InNamespace(ns.Name))
		if err != nil {
			continue
		}

		// 3. 데이터 가공
		payload := map[string]interface{}{
			"namespace":   ns.Name,
			"services":    service.ExtractServiceInfo(svcList),
			"deployments": service.ExtractDeploymentInfo(deployList),
		}

		// 4. REST API 호출
		if err := service.SendMetric(payload); err != nil {
			// 재시도 로직 추가 가능
		}
	}
	return nil
}
