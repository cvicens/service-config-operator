package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type VolumeMount struct {
	// This must match the Name of a Volume.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Mounted read-only if true, read-write otherwise (false or unspecified).
	// Defaults to false.
	// +optional
	ReadOnly bool `json:"readOnly,omitempty" protobuf:"varint,2,opt,name=readOnly"`
	// Path within the container at which the volume should be mounted.  Must
	// not contain ':'.
	MountPath string `json:"mountPath" protobuf:"bytes,3,opt,name=mountPath"`
	// Path within the volume from which the container's volume should be mounted.
	// Defaults to "" (volume's root).
	// +optional
	SubPath string `json:"subPath,omitempty" protobuf:"bytes,4,opt,name=subPath"`
	// mountPropagation determines how mounts are propagated from the host
	// to container and the other way around.
	// When not set, MountPropagationNone is used.
	// This field is beta in 1.10.
	// +optional
	MountPropagation *corev1.MountPropagationMode `json:"mountPropagation,omitempty" protobuf:"bytes,5,opt,name=mountPropagation,casttype=MountPropagationMode"`
}

// ServiceConfigSpec defines the desired state of ServiceConfig
// +k8s:openapi-gen=true
type ServiceConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Enabled           bool              `json:"enabled"`
	GitUrl            string            `json:"gitUrl"`
	GitRef            string            `json:"gitRef"`
	DescriptorsFolder string            `json:"descriptorsFolder"`
	ServiceName       string            `json:"serviceName"`
	MinReplicas       int64             `json:"minReplicas"`
	Image             string            `json:"image,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Env               []corev1.EnvVar   `json:"env,omitempty"`
	LivenessProbe     *corev1.Probe     `json:"livenessProbe,omitempty"`
	ReadinessProbe    *corev1.Probe     `json:"readinessProbe,omitempty"`
	VolumeMounts      []VolumeMount     `json:"volumeMounts,omitempty"`
}

// ServiceConfigStatus defines the observed state of ServiceConfig
// +k8s:openapi-gen=true
type ServiceConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceConfig is the Schema for the serviceconfigs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type ServiceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceConfigSpec   `json:"spec,omitempty"`
	Status ServiceConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceConfigList contains a list of ServiceConfig
type ServiceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceConfig{}, &ServiceConfigList{})
}
