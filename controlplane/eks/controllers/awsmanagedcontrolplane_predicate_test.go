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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	ekscontrolplanev1 "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestAWSManagedControlPlaneStatusUpdatePredicate(t *testing.T) {
	base := &ekscontrolplanev1.AWSManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cp",
			Namespace:       "default",
			Generation:      1,
			ResourceVersion: "100",
		},
		Spec: ekscontrolplanev1.AWSManagedControlPlaneSpec{
			EKSClusterName: "cp",
			Region:         "us-east-1",
		},
	}

	tests := []struct {
		name    string
		mutate  func(o *ekscontrolplanev1.AWSManagedControlPlane)
		enqueue bool
	}{
		{
			name: "status-only change is filtered out",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				o.Status.Ready = true
				o.ResourceVersion = "101"
				o.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "capa"}}
			},
			enqueue: false,
		},
		{
			name:    "resync event with identical object triggers reconcile",
			mutate:  func(o *ekscontrolplanev1.AWSManagedControlPlane) {},
			enqueue: true,
		},
		{
			name: "spec change triggers reconcile",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				o.Status.Ready = true
				o.ResourceVersion = "101"
				o.Spec.Version = ptr.To("1.30")
			},
			enqueue: true,
		},
		{
			name: "generation bump triggers reconcile",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				o.Generation = 2
				o.ResourceVersion = "101"
			},
			enqueue: true,
		},
		{
			name: "deletion timestamp triggers reconcile",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				now := metav1.Now()
				o.DeletionTimestamp = &now
				o.ResourceVersion = "101"
			},
			enqueue: true,
		},
		{
			name: "annotation change triggers reconcile",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				o.Annotations = map[string]string{"cluster.x-k8s.io/paused": "true"}
				o.ResourceVersion = "101"
			},
			enqueue: true,
		},
		{
			name: "finalizer change triggers reconcile",
			mutate: func(o *ekscontrolplanev1.AWSManagedControlPlane) {
				o.Finalizers = []string{"foo"}
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

			result := awsManagedControlPlaneStatusUpdatePredicate.Update(event.UpdateEvent{
				ObjectOld: oldObj,
				ObjectNew: newObj,
			})
			g.Expect(result).To(Equal(tc.enqueue))
		})
	}
}

func TestAWSManagedControlPlaneStatusUpdatePredicateOtherKind(t *testing.T) {
	g := NewWithT(t)

	result := awsManagedControlPlaneStatusUpdatePredicate.Update(event.UpdateEvent{
		ObjectOld: &clusterv1.Cluster{},
		ObjectNew: &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}},
	})
	g.Expect(result).To(BeTrue())
}
