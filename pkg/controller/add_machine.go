/*
Copyright 2018 The Kubernetes authors.

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

package controller

import (
	"github.com/uccps-samples/machine-api-operator/pkg/controller/machine"
	ocm "sigs.k8s.io/cluster-api-provider-openstack/pkg/cloud/openstack/machine"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, func(m manager.Manager) error {
		params := getActuatorParams(m)
		machineActuator, err := ocm.NewActuator(params)
		if err != nil {
			return err
		}
		return machine.AddWithActuator(m, machineActuator)
	})
}
