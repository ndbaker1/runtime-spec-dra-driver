package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	GroupName = "dra.runtime-spec.io"
	Version   = "v1alpha1"

	RuntimeSpecEditConfigKind = "RuntimeSpecEditConfig"
)

// Decoder implements a decoder for objects in this API group.
var Decoder runtime.Decoder

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RuntimeSpecEditConfig struct {
	metav1.TypeMeta `json:",inline"`
	Spec            runtime.RawExtension `json:"spec,omitempty"`
}

func (c *RuntimeSpecEditConfig) Normalize() error {
	if c == nil {
		return fmt.Errorf("config is 'nil'")
	}
	return nil
}

func init() {
	// Create a new scheme and add our types to it. If at some point in the
	// future a new version of the configuration API becomes necessary, then
	// conversion functions can be generated and registered to continue
	// supporting older versions.
	scheme := runtime.NewScheme()
	schemeGroupVersion := schema.GroupVersion{
		Group:   GroupName,
		Version: Version,
	}
	scheme.AddKnownTypes(schemeGroupVersion,
		&RuntimeSpecEditConfig{},
	)
	metav1.AddToGroupVersion(scheme, schemeGroupVersion)

	Decoder = json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		scheme,
		scheme,
		json.SerializerOptions{
			Pretty: true, Strict: true,
		},
	)
}
