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

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestAWSMachineStatusUpdatePredicate(t *testing.T) {
	base := &infrav1.AWSMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "m",
			Namespace:       "default",
			Generation:      1,
			ResourceVersion: "100",
		},
		Spec: infrav1.AWSMachineSpec{InstanceType: "t3.large"},
	}

	tests := []struct {
		name    string
		mutate  func(o *infrav1.AWSMachine)
		enqueue bool
	}{
		{
			name: "status-only change is filtered out",
			mutate: func(o *infrav1.AWSMachine) {
				o.Status.Ready = true
				o.ResourceVersion = "101"
				o.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "capa"}}
			},
			enqueue: false,
		},
		{
			name:    "resync event with identical object triggers reconcile",
			mutate:  func(o *infrav1.AWSMachine) {},
			enqueue: true,
		},
		{
			name: "spec change triggers reconcile",
			mutate: func(o *infrav1.AWSMachine) {
				o.Status.Ready = true
				o.ResourceVersion = "101"
				o.Spec.InstanceType = "t3.xlarge"
			},
			enqueue: true,
		},
		{
			name: "deletion timestamp triggers reconcile",
			mutate: func(o *infrav1.AWSMachine) {
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

			result := awsMachineStatusUpdatePredicate.Update(event.UpdateEvent{
				ObjectOld: oldObj,
				ObjectNew: newObj,
			})
			g.Expect(result).To(Equal(tc.enqueue))
		})
	}
}

func TestAWSMachineStatusUpdatePredicateOtherKind(t *testing.T) {
	g := NewWithT(t)

	result := awsMachineStatusUpdatePredicate.Update(event.UpdateEvent{
		ObjectOld: &clusterv1.Cluster{},
		ObjectNew: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}},
	})
	g.Expect(result).To(BeTrue())
}

// TestKindStringGuardIsANoOp demonstrates why the predicate uses a type assertion rather
// than a TypeMeta Kind comparison. Objects delivered from the controller cache have an
// empty Kind (verified against the cache via envtest), so a Kind-based guard never matches
// and the predicate degrades to "reconcile on everything" — it does not filter status-only
// updates as intended.
func TestKindStringGuardIsANoOp(t *testing.T) {
	g := NewWithT(t)

	// Typed objects, as the cache delivers them, carry no TypeMeta.
	oldObj := &infrav1.AWSMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "m", ResourceVersion: "100"},
	}
	g.Expect(oldObj.GetObjectKind().GroupVersionKind().Kind).To(BeEmpty())

	// A status-only update: the predicate is meant to filter this (no reconcile).
	newObj := oldObj.DeepCopy()
	newObj.Status.Ready = true
	newObj.ResourceVersion = "101"

	updateEvent := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}

	// The old, Kind-string guarded form: short-circuits on the empty Kind and never
	// reaches the comparison, so it (incorrectly) returns true — a no-op.
	kindStringGuarded := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind != "AWSMachine" {
				return true
			}
			oldM := e.ObjectOld.(*infrav1.AWSMachine).DeepCopy()
			newM := e.ObjectNew.(*infrav1.AWSMachine).DeepCopy()
			oldM.Status = infrav1.AWSMachineStatus{}
			newM.Status = infrav1.AWSMachineStatus{}
			oldM.ResourceVersion = ""
			newM.ResourceVersion = ""
			return !cmp.Equal(oldM, newM)
		},
	}
	g.Expect(kindStringGuarded.Update(updateEvent)).To(BeTrue(),
		"kind-string guard should fail to filter (no-op) on an empty-Kind object")

	// The type-assertion form correctly filters the status-only update.
	g.Expect(awsMachineStatusUpdatePredicate.Update(updateEvent)).To(BeFalse(),
		"type-assertion guard should filter the status-only update")
}
