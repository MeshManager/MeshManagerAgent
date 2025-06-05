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
	"fmt"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshmanagerv1 "github.com/MeshManager/MeshManagerAgent.git/api/v1"

	generator "github.com/MeshManager/MeshManagerAgent.git/internal/controller/generators"
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
	logger := log.FromContext(ctx)

	//나중에 아래 fmt.Print 지울 것
	fmt.Print("yaml 변경됨")

	var istioRoute meshmanagerv1.IstioRoute
	if err := r.Get(ctx, req.NamespacedName, &istioRoute); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	_ = istioRoute.DeepCopy()

	for _, svcConfig := range istioRoute.Spec.Services {
		vs := generator.GenerateVirtualService(svcConfig)
		if err := ctrl.SetControllerReference(&istioRoute, vs, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.CreateOrUpdate(ctx, vs); err != nil {
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
			//if err := ctrl.SetControllerReference(&istioRoute, ef, r.Scheme); err != nil {
			//	return ctrl.Result{}, err
			//}
			if err := r.CreateOrUpdate(ctx, ef); err != nil {
				logger.Error(err, "failed to manage EnvoyFilter")
				return ctrl.Result{}, err
			}
		}

	}

	//TODO Update Status

	return ctrl.Result{}, nil
}

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
