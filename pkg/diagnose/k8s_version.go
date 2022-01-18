/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

package diagnose

import (
	"github.com/submariner-io/submariner-operator/pkg/cluster"
	"github.com/submariner-io/submariner-operator/pkg/reporter"
	"github.com/submariner-io/submariner-operator/pkg/version"
)

func K8sVersion(clusterInfo *cluster.Info, status reporter.Interface) bool {
	status.Start("Checking Submariner support for the Kubernetes version")
	defer status.End()

	k8sVersion, failedRequirements, err := version.CheckRequirements(clusterInfo.ClientProducer.ForKubernetes())
	if err != nil {
		status.Failure(err.Error())

		return false
	}

	failed := false

	for i := range failedRequirements {
		status.Failure(failedRequirements[i])

		failed = true
	}

	if failed {
		return false
	}

	status.Success("Kubernetes version %q is supported", k8sVersion)

	return true
}