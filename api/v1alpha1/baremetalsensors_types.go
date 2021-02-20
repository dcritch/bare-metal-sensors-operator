/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BareMetalSensorsSpec defines the desired state of BareMetalSensors
type BareMetalSensorsSpec struct {
	// InfluxHost is the hostname or IP of an existing InfluxDB server
	InfluxHost string `json:"influxhost"`
}

// BareMetalSensorsStatus defines the observed state of BareMetalSensors
type BareMetalSensorsStatus struct {
	// Nodes are the names of the bare-metal-sensors pods
	Nodes []string `json:"nodes"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BareMetalSensors is the Schema for the baremetalsensors API
// +kubebuilder:subresource:status
type BareMetalSensors struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalSensorsSpec   `json:"spec,omitempty"`
	Status BareMetalSensorsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalSensorsList contains a list of BareMetalSensors
type BareMetalSensorsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalSensors `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalSensors{}, &BareMetalSensorsList{})
}
