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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IstioRouteSpec defines the desired state of IstioRoute
type IstioRouteSpec struct {
	Services []ServiceConfig `json:"services"`
}

type ServiceConfig struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Type      ServiceType `json:"type"` // Canary, StickyCanary

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	CommitHashes []string `json:"commitHashes,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:nullable
	Ratio *int `json:"ratio,omitempty"`

	Dependencies []Dependency `json:"dependencies,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	SessionDuration int `json:"sessionDuration,omitempty"`

	OutlierDetection *OutlierDetection `json:"outlierDetection,omitempty"`

	// +kubebuilder:validation:Optional
	DarknessReleases []DarknessRelease `json:"darknessReleases,omitempty"`
}

type ServiceType string

const (
	StandardType     ServiceType = "Standard"
	CanaryType       ServiceType = "Canary"
	StickyCanaryType ServiceType = "StickyCanary"
)

type OutlierDetection struct {
	// +kubebuilder:validation:Minimum=0
	Consecutive5xxErrors int `json:"consecutive5xxErrors,omitempty"`

	// +kubebuilder:validation:Minimum=0
	ConsecutiveGatewayErrors int `json:"consecutiveGatewayErrors,omitempty"`

	// +kubebuilder:validation:Pattern=`^([0-9]+(s|m|h))+$`
	Interval string `json:"interval,omitempty"`
}

type Dependency struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	CommitHashes []string `json:"commitHashes"`
}

type DarknessRelease struct {
	// +kubebuilder:validation:Required
	CommitHash string `json:"commitHash"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxItems=10
	IPs []string `json:"ips,omitempty"`
}

// IstioRouteStatus defines the observed state of IstioRoute
type IstioRouteStatus struct {
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
	LastAppliedHash string             `json:"lastAppliedHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IstioRoute is the Schema for the istioroutes API
type IstioRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioRouteSpec   `json:"spec,omitempty"`
	Status IstioRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioRouteList contains a list of IstioRoute
type IstioRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioRoute{}, &IstioRouteList{})
}
