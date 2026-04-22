package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type KafkaTopicSpec struct {
	Partitions        int32             `json:"partitions"`
	ReplicationFactor int32             `json:"replicationFactor"`
	Config            map[string]string `json:"config,omitempty"`
}

type KafkaTopic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KafkaTopicSpec `json:"spec,omitempty"`
}

type KafkaTopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaTopic `json:"items"`
}

func (in *KafkaTopic) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(KafkaTopic)
	*out = *in

	// deep copy map
	if in.Spec.Config != nil {
		out.Spec.Config = make(map[string]string, len(in.Spec.Config))
		for k, v := range in.Spec.Config {
			out.Spec.Config[k] = v
		}
	}

	return out
}

func (in *KafkaTopicList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(KafkaTopicList)
	*out = *in

	if in.Items != nil {
		out.Items = make([]KafkaTopic, len(in.Items))
		copy(out.Items, in.Items)
	}

	return out
}
