/*
Copyright 2026 The Kubernetes Authors.

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

package predicates

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestStatusOnlyUpdateFilter(t *testing.T) {
	statusOnlyUpdateFilter := StatusOnlyUpdateFilter(func(node *corev1.Node) *corev1.Node {
		node = node.DeepCopy()
		node.Status = corev1.NodeStatus{}
		return node
	})

	baseNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "node",
			ResourceVersion: "100",
		},
		Spec: corev1.NodeSpec{PodCIDR: "10.0.0.0/24"},
	}

	tests := []struct {
		name    string
		event   event.UpdateEvent
		enqueue bool
	}{
		{
			name: "status-only update is filtered out",
			event: func() event.UpdateEvent {
				oldNode := baseNode.DeepCopy()
				newNode := baseNode.DeepCopy()
				newNode.Status.Phase = corev1.NodeRunning
				newNode.ResourceVersion = "101"
				newNode.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "capa"}}
				return event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}
			}(),
			enqueue: false,
		},
		{
			name: "spec change is enqueued",
			event: func() event.UpdateEvent {
				oldNode := baseNode.DeepCopy()
				newNode := baseNode.DeepCopy()
				newNode.Status.Phase = corev1.NodeRunning
				newNode.ResourceVersion = "101"
				newNode.Spec.PodCIDR = "10.0.1.0/24"
				return event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}
			}(),
			enqueue: true,
		},
		{
			name: "resync is enqueued",
			event: func() event.UpdateEvent {
				oldNode := baseNode.DeepCopy()
				return event.UpdateEvent{ObjectOld: oldNode, ObjectNew: oldNode.DeepCopy()}
			}(),
			enqueue: true,
		},
		{
			name: "different kind is enqueued",
			event: event.UpdateEvent{
				ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "100"}},
				ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "101"}},
			},
			enqueue: true,
		},
		{
			name: "deletion timestamp is enqueued",
			event: func() event.UpdateEvent {
				oldNode := baseNode.DeepCopy()
				newNode := baseNode.DeepCopy()
				now := metav1.Now()
				newNode.DeletionTimestamp = &now
				newNode.ResourceVersion = "101"
				return event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode}
			}(),
			enqueue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(statusOnlyUpdateFilter.Update(tt.event)).To(Equal(tt.enqueue))
		})
	}
}
