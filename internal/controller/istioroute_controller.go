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
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent/api/v1"

	generator "github.com/MeshManager/MeshManagerAgent/internal/controller/generators"
)

const EnvoyFilterFinalizer = "meshmanager.com/envoyfilter-cleanup"

// IstioRouteReconciler reconciles a IstioRoute object
type IstioRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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
	logger := log.FromContext(ctx)

	//TODO 나중에 아래 fmt.Print 지울 것
	// fmt.Print("yaml 변경됨")

	var istioRoute meshmanagerv1.IstioRoute
	if err := r.Get(ctx, req.NamespacedName, &istioRoute); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	_ = istioRoute.DeepCopy()

	gateway := generator.GenerateIstioGateway("istio-gateway", "default")
	if err := ctrl.SetControllerReference(&istioRoute, gateway, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.CreateOrUpdate(ctx, gateway); err != nil {
		logger.Error(err, "failed to manage Gateway")
		return ctrl.Result{}, err
	}

	for _, svcConfig := range istioRoute.Spec.Services {
		vs := generator.GenerateVirtualService(svcConfig)
		if err := ctrl.SetControllerReference(&istioRoute, vs, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.CreateOrUpdate(ctx, vs); err != nil {
			logger.Error(err, "failed to manage VirtualService")
			return ctrl.Result{}, err
		}

		ingressVS := generator.GenerateIngressVirtualService(svcConfig)
		if err := ctrl.SetControllerReference(&istioRoute, ingressVS, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.CreateOrUpdate(ctx, ingressVS); err != nil {
			logger.Error(err, "failed to manage VirtualService")
			return ctrl.Result{}, err
		}

		dr := generator.GenerateDestinationRule(svcConfig)
		if err := ctrl.SetControllerReference(&istioRoute, dr, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.CreateOrUpdate(ctx, dr); err != nil {
			logger.Error(err, "failed to manage DestinationRule")
			return ctrl.Result{}, err
		}

		if svcConfig.Type == meshmanagerv1.CanaryType || svcConfig.Type == meshmanagerv1.StickyCanaryType {
			ef := generator.GenerateEnvoyFilter(svcConfig, &istioRoute)

			logger.Info("Envoy 생성 루틴 시작")

			//if err := ctrl.SetControllerReference(&istioRoute, ef, r.Scheme); err != nil {
			//	return ctrl.Result{}, err
			//}
			if err := r.CreateOrUpdate(ctx, ef); err != nil {
				logger.Error(err, "failed to manage EnvoyFilter")
				return ctrl.Result{}, err
			}
		}

	}

	// Finalizer 추가
	if !controllerutil.ContainsFinalizer(&istioRoute, EnvoyFilterFinalizer) {
		controllerutil.AddFinalizer(&istioRoute, EnvoyFilterFinalizer)
		if err := r.Update(ctx, &istioRoute); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 삭제 처리
	if !istioRoute.DeletionTimestamp.IsZero() {
		if err := r.CleanupEnvoyFilters(ctx, &istioRoute); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(&istioRoute, EnvoyFilterFinalizer)
		return ctrl.Result{}, r.Update(ctx, &istioRoute)
	}

	//TODO Update Status

	return ctrl.Result{}, nil
}

// CreateOrUpdate to apply k8s resource
func (r *IstioRouteReconciler) CreateOrUpdate(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	existing := obj.DeepCopyObject().(client.Object)

	if err := r.Get(ctx, key, existing); err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, obj)
		}
		return err
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshmanagerv1.IstioRoute{}).
		Owns(&istiov1beta1.VirtualService{}).
		Owns(&istiov1beta1.DestinationRule{}).
		Owns(&istiov1beta1.EnvoyFilter{}).
		Complete(r)
}

// CleanupEnvoyFilters to clean up envoy filter resource, For finalizer
func (r *IstioRouteReconciler) CleanupEnvoyFilters(ctx context.Context, ir *meshmanagerv1.IstioRoute) error {
	logger := log.FromContext(ctx)
	logger.Info("EnvoyFilter 정리 시작", "istioroute", ir.Name)

	envoyFilterList := &istiov1beta1.EnvoyFilterList{}
	listOpts := []client.ListOption{
		client.InNamespace("istio-system"),
		client.MatchingLabels{
			"managed-by":      "istioroute-controller",
			"istioroute-type": "envoy-filter",
		},
	}

	if err := r.List(ctx, envoyFilterList, listOpts...); err != nil {
		logger.Error(err, "EnvoyFilter 리스트 조회 실패")
		return err
	}

	for i := range envoyFilterList.Items {
		ef := envoyFilterList.Items[i]

		logger.Info("삭제 대상 EnvoyFilter 정보",
			"name", ef.Name,
			"namespace", ef.Namespace,
			"labels", ef.Labels,
			"annotations", ef.Annotations,
		)

		if err := r.Delete(ctx, ef); err != nil {
			logger.Error(err, "EnvoyFilter 삭제 실패", "name", ef.Name)
		}
	}
	return nil
}
