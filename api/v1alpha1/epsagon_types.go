package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EpsagonSpec defines the desired state of Epsagon
type EpsagonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// EpsagonToken is the Epsagon token for the account integrating this cluster
	EpsagonToken string `json:"epsagonToken"`

	// ClusterEndpoint cluster api endpoint to access from outside of the cluster
	ClusterEndpoint string `json:"clusterEndpoint"`
}

// EpsagonStatus defines the observed state of Epsagon
type EpsagonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Status of the integration with Epsagon
	Status string `json:"status,omitempty"`
	// Reason description of the error, if any.
	Reason string `json:"reason,omitempty"`
	// LastUpdate the last update time of this resource
	LastUpdate metav1.Time `json:"lastUpdate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Epsagon is the Schema for the epsagons API
type Epsagon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EpsagonSpec   `json:"spec,omitempty"`
	Status EpsagonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EpsagonList contains a list of Epsagon
type EpsagonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Epsagon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Epsagon{}, &EpsagonList{})
}
