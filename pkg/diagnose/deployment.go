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
	"context"

	"github.com/submariner-io/submariner-operator/internal/constants"
	"github.com/submariner-io/submariner-operator/pkg/cluster"
	"github.com/submariner-io/submariner-operator/pkg/reporter"
	"github.com/submariner-io/submariner/pkg/cidr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Deployments(clusterInfo *cluster.Info, status reporter.Interface) bool {
	if clusterInfo.Submariner == nil {
		status.Warning(constants.SubmMissingMessage)

		return true
	}

	return checkOverlappingCIDRs(clusterInfo, status) && checkPods(clusterInfo, status)
}

func checkOverlappingCIDRs(clusterInfo *cluster.Info, status reporter.Interface) bool {
	if clusterInfo.Submariner.Spec.GlobalCIDR != "" {
		status.Start("Globalnet deployment detected - checking if globalnet CIDRs overlap")
	} else {
		status.Start("Non-Globalnet deployment detected - checking if cluster CIDRs overlap")
	}

	defer status.End()

	endpointList, err := clusterInfo.ClientProducer.ForSubmariner().SubmarinerV1().Endpoints(clusterInfo.Submariner.Namespace).
		List(context.TODO(),
			metav1.ListOptions{})
	if err != nil {
		status.Failure("Error listing the Submariner endpoints: %v", err)

		return false
	}

	failed := false

	for i := range endpointList.Items {
		source := &endpointList.Items[i]

		destEndpoints := endpointList.Items[i+1:]
		for j := range destEndpoints {
			dest := &destEndpoints[j]

			// Currently, we don't support multiple endpoints in a cluster, hence return an error.
			// When the corresponding support is added, this check needs to be updated.
			if source.Spec.ClusterID == dest.Spec.ClusterID {
				status.Failure("Found multiple Submariner endpoints (%q and %q) in cluster %q",
					source.Name, dest.Name, source.Spec.ClusterID)

				failed = true

				continue
			}

			for _, subnet := range dest.Spec.Subnets {
				overlap, err := cidr.IsOverlapping(source.Spec.Subnets, subnet)
				if err != nil {
					// Ideally this case will never hit, as the subnets are valid CIDRs
					status.Failure("Error parsing CIDR in cluster %q: %s", dest.Spec.ClusterID, err)

					failed = true

					continue
				}

				if overlap {
					status.Failure("CIDR %q in cluster %q overlaps with cluster %q (CIDRs: %v)",
						subnet, dest.Spec.ClusterID, source.Spec.ClusterID, source.Spec.Subnets)

					failed = true
				}
			}
		}
	}

	if failed {
		return false
	}

	if clusterInfo.Submariner.Spec.GlobalCIDR != "" {
		status.Success("Clusters do not have overlapping globalnet CIDRs")
	} else {
		status.Success("Clusters do not have overlapping CIDRs")
	}

	return true
}

func checkPods(clusterInfo *cluster.Info, status reporter.Interface) bool {
	status.Start("Checking Submariner pods")
	defer status.End()

	deploymentFailed := false
	deamonSetFailed := false

	submGWErr := checkDaemonset(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace, "submariner-gateway", status)
	submRAErr := checkDaemonset(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace, "submariner-routeagent", status)

	// Check if service-discovery components are deployed and running if enabled
	if clusterInfo.Submariner.Spec.ServiceDiscoveryEnabled {
		lhAgentErr := checkDeployment(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace,
			"submariner-lighthouse-agent", status)
		lhCorednsErr := checkDeployment(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace,
			"submariner-lighthouse-coredns", status)

		if lhAgentErr || lhCorednsErr {
			deploymentFailed = true
		}
	}

	// Check if globalnet components are deployed and running if enabled
	if clusterInfo.Submariner.Spec.GlobalCIDR != "" {
		submGNErr := checkDaemonset(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace, "submariner-globalnet", status)
		if submGNErr || submRAErr || submGWErr {
			deamonSetFailed = true
		}
	}

	podFailed := checkPodsStatus(clusterInfo.ClientProducer.ForKubernetes(), constants.OperatorNamespace, status)

	if deploymentFailed || deamonSetFailed || podFailed {
		return false
	}

	status.Success("All Submariner pods are up and running")

	return true
}

func checkDeployment(k8sClient kubernetes.Interface, namespace, deploymentName string, status reporter.Interface) bool {
	deployment, err := k8sClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		status.Failure("Error obtaining Deployment %q: %v", deploymentName, err)
		return true
	}

	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	if deployment.Status.AvailableReplicas != replicas {
		status.Failure("The desired number of replicas for Deployment %q (%d)"+
			" does not match the actual number running (%d)", deploymentName, replicas,
			deployment.Status.AvailableReplicas)

		return true
	}

	return false
}

func checkDaemonset(k8sClient kubernetes.Interface, namespace, daemonSetName string, status reporter.Interface) bool {
	daemonSet, err := k8sClient.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		status.Failure("Error obtaining Daemonset %q: %v", daemonSetName, err)
		return true
	}

	if daemonSet.Status.CurrentNumberScheduled != daemonSet.Status.DesiredNumberScheduled {
		status.Failure("The desired number of running pods for DaemonSet %q (%d)"+
			" does not match the actual number (%d)", daemonSetName, daemonSet.Status.DesiredNumberScheduled,
			daemonSet.Status.CurrentNumberScheduled)

		return true
	}

	return false
}

func checkPodsStatus(k8sClient kubernetes.Interface, namespace string, status reporter.Interface) bool {
	pods, err := k8sClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.Failure("Error obtaining Pods list: %v", err)
		return true
	}

	failure := false

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != v1.PodRunning {
			status.Failure("Pod %q is not running. (current state is %v)", pod.Name, pod.Status.Phase)

			failure = true

			continue
		}

		for j := range pod.Status.ContainerStatuses {
			c := &pod.Status.ContainerStatuses[j]
			if c.RestartCount >= 5 {
				status.Warning("Pod %q has restarted %d times", pod.Name, c.RestartCount)
			}
		}
	}

	return failure
}