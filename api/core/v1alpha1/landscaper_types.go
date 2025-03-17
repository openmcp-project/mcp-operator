package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LandscaperConfiguration contains the configuration which is required for setting up a LaaS instance.
type LandscaperConfiguration struct {
	// Deployers is the list of deployers that should be installed.
	// +optional
	Deployers []string `json:"deployers,omitempty"`
}

// ExternalLandscaperStatus contains the status of a LaaS instance.
type ExternalLandscaperStatus struct {
}

// LandscaperStatus contains the landscaper status and potentially other fields which should not be exposed to the customer.
type LandscaperStatus struct {
	CommonComponentStatus     `json:",inline"`
	*ExternalLandscaperStatus `json:",inline"`

	// LandscaperDeploymentInfo contains information about the corresponding LandscaperDeployment resource.
	// +optional
	LandscaperDeploymentInfo *LandscaperDeploymentInfo `json:"landscaperDeployment,omitempty"`
}

// LandscaperDeploymentInfo contains information about the corresponding Landscaper deployment resource.
type LandscaperDeploymentInfo struct {
	// Name is the name of the Landscaper deployment.
	Name string `json:"name"`
	// Namespace is the namespace of the Landscaper deployment.
	Namespace string `json:"namespace"`
}

// LandscaperSpec contains the Landscaper configuration and potentially other fields which should not be exposed to the customer.
type LandscaperSpec struct {
	LandscaperConfiguration `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Landscaper is the Schema for the laasinstances API
// +kubebuilder:resource:shortName=ls
// +kubebuilder:printcolumn:name="Successfully_Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="LandscaperReconciliation")].status`
// +kubebuilder:printcolumn:name="Deleted",type="date",JSONPath=".metadata.deletionTimestamp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Landscaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LandscaperSpec   `json:"spec,omitempty"`
	Status LandscaperStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LandscaperList contains a list of Landscaper
type LandscaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Landscaper `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Landscaper{}, &LandscaperList{})
}
