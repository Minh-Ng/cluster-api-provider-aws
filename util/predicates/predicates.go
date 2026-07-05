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

// Package predicates implements controller-runtime predicates shared across CAPA controllers.
package predicates

import (
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// StatusOnlyUpdateFilter returns a predicate that filters out update events that only
// change a T's status, so a controller does not re-reconcile in response to its own
// status patches. Changes to spec, deletionTimestamp, annotations or finalizers still
// trigger reconciliation, as do events for kinds other than T.
//
// Periodic resync events (which deliver the same object as both old and new) pass
// through: sync-period reconciliation — EKS kubeconfig token refresh, drift detection —
// depends on them, and a real update always bumps ResourceVersion.
//
// clearStatusOnCopy must return a deep copy of the object with its status zeroed; the
// remaining fields that change on every status write are cleared here.
//
// Note: a type assertion is used (not a TypeMeta Kind comparison) because objects
// delivered from the controller cache have an empty Kind, which would make a Kind-based
// guard a no-op.
func StatusOnlyUpdateFilter[T client.Object](clearStatusOnCopy func(T) T) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, ok := e.ObjectOld.(T)
			if !ok {
				return true
			}
			newObj, ok := e.ObjectNew.(T)
			if !ok {
				return true
			}

			if oldObj.GetResourceVersion() == newObj.GetResourceVersion() {
				// Resync, not a real update.
				return true
			}

			oldCopy := clearStatusOnCopy(oldObj)
			newCopy := clearStatusOnCopy(newObj)

			// Zero out the remaining fields that change on every status write so the
			// comparison reflects only spec/metadata differences.
			oldCopy.SetResourceVersion("")
			newCopy.SetResourceVersion("")
			oldCopy.SetManagedFields(nil)
			newCopy.SetManagedFields(nil)

			return !cmp.Equal(oldCopy, newCopy)
		},
	}
}
