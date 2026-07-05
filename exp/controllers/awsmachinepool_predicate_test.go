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

package controllers

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	expinfrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/exp/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestAWSMachinePoolStatusUpdatePredicate(t *testing.T) {
	base := &expinfrav1.AWSMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "pool",
			Namespace:       "default",
			Generation:      1,
			ResourceVersion: "100",
		},
		Spec: expinfrav1.AWSMachinePoolSpec{MinSize: 1, MaxSize: 3},
	}

	tests := []struct {
		name    string
		mutate  func(o *expinfrav1.AWSMachinePool)
		enqueue bool
	}{
		{
			name: "status-only change is filtered out",
			mutate: func(o *expinfrav1.AWSMachinePool) {
				o.Status.Ready = true
				o.Status.Replicas = 3
				o.ResourceVersion = "101"
				o.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "capa"}}
			},
			enqueue: false,
		},
		{
			name:    "resync event with identical object triggers reconcile",
			mutate:  func(o *expinfrav1.AWSMachinePool) {},
			enqueue: true,
		},
		{
			name: "spec change triggers reconcile",
			mutate: func(o *expinfrav1.AWSMachinePool) {
				o.Status.Replicas = 3
				o.ResourceVersion = "101"
				o.Spec.MaxSize = 5
			},
			enqueue: true,
		},
		{
			name: "deletion timestamp triggers reconcile",
			mutate: func(o *expinfrav1.AWSMachinePool) {
				now := metav1.Now()
				o.DeletionTimestamp = &now
				o.ResourceVersion = "101"
			},
			enqueue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			oldObj := base.DeepCopy()
			newObj := base.DeepCopy()
			tc.mutate(newObj)

			result := awsMachinePoolStatusUpdatePredicate.Update(event.UpdateEvent{
				ObjectOld: oldObj,
				ObjectNew: newObj,
			})
			g.Expect(result).To(Equal(tc.enqueue))
		})
	}
}

func TestAWSMachinePoolStatusUpdatePredicateOtherKind(t *testing.T) {
	g := NewWithT(t)

	result := awsMachinePoolStatusUpdatePredicate.Update(event.UpdateEvent{
		ObjectOld: &clusterv1.MachinePool{},
		ObjectNew: &clusterv1.MachinePool{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}},
	})
	g.Expect(result).To(BeTrue())
}
