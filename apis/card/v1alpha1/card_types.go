/*
Copyright 2020 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// CardParameters are the configurable fields of a Card.
type CardParameters struct{}

// CardObservation are the observable fields of a Card.
type CardObservation struct {
	Suit string `json:"suit,omitempty"`
	Rank string `json:"rank,omitempty"`
	Face string `json:"face,omitempty"`
}

// A CardSpec defines the desired state of a Card.
type CardSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       CardParameters `json:"forProvider"`
}

// A CardStatus represents the observed state of a Card.
type CardStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          CardObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Card is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,template}
type Card struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CardSpec   `json:"spec"`
	Status CardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CardList contains a list of Card
type CardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Card `json:"items"`
}

// Card type metadata.
var (
	CardKind             = reflect.TypeOf(Card{}).Name()
	CardGroupKind        = schema.GroupKind{Group: Group, Kind: CardKind}.String()
	CardKindAPIVersion   = CardKind + "." + SchemeGroupVersion.String()
	CardGroupVersionKind = SchemeGroupVersion.WithKind(CardKind)
)

func init() {
	SchemeBuilder.Register(&Card{}, &CardList{})
}
