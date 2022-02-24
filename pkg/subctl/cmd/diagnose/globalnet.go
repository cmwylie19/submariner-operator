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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/submariner-io/submariner-operator/internal/cli"
	"github.com/submariner-io/submariner-operator/pkg/reporter"
	"github.com/submariner-io/submariner-operator/pkg/subctl/cmd"
	submarinerv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	"github.com/submariner-io/submariner/pkg/globalnet/constants"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mcsClientset "sigs.k8s.io/mcs-api/pkg/client/clientset/versioned"
)

func init() {
	diagnoseCmd.AddCommand(&cobra.Command{
		Use:   "globalnet",
		Short: "Check globalnet configuration",
		Long:  "This command checks globalnet configuration",
		Run: func(command *cobra.Command, args []string) {
			cmd.ExecuteMultiCluster(restConfigProducer, checkGlobalnet)
		},
	})
}

func checkGlobalnet(cluster *cmd.Cluster) bool {
	status := cli.NewStatus()

	if cluster.Submariner == nil {
		status.Warning(cmd.SubmMissingMessage)
		return true
	}

	if cluster.Submariner.Spec.GlobalCIDR == "" {
		status.Success("Globalnet is not installed - skipping")
		return true
	}

	status.Start("Checking Globalnet configuration")
	defer status.End()

	retValue := checkClusterGlobalEgressIps(cluster, status) &&
		checkGlobalEgressIps(cluster, status) &&
		checkGlobalIngressIps(cluster, status)

	if retValue {
		status.EndWithSuccess("Globalnet is enabled and properly configured")
	}

	return retValue
}

func checkClusterGlobalEgressIps(cluster *cmd.Cluster, status reporter.Interface) bool {
	clusterGlobalEgress, err := cluster.SubmClient.SubmarinerV1().ClusterGlobalEgressIPs(
		corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.Warning("Error listing the ClusterGlobalEgressIP resources: %v", err)
		return false
	}

	if len(clusterGlobalEgress.Items) != 1 {
		status.Warning(
			"Found %d ClusterGlobalEgressIP resources but only the default instance (%s) is supported",
			len(clusterGlobalEgress.Items), constants.ClusterGlobalEgressIPName)
	}

	foundDefaultResource := false
	index := 0

	for index = range clusterGlobalEgress.Items {
		if clusterGlobalEgress.Items[index].Name == constants.ClusterGlobalEgressIPName {
			foundDefaultResource = true
			break
		}
	}

	if !foundDefaultResource {
		status.Failure("Couldn't find the default ClusterGlobalEgressIP resource(%s)", constants.ClusterGlobalEgressIPName)
		return false
	}

	clusterGlobalEgressIP := clusterGlobalEgress.Items[index]

	retValue := true
	numberOfIPs := -1

	if clusterGlobalEgressIP.Spec.NumberOfIPs != nil {
		numberOfIPs = *clusterGlobalEgressIP.Spec.NumberOfIPs
	}

	if numberOfIPs != len(clusterGlobalEgressIP.Status.AllocatedIPs) {
		status.Failure("The number of requested IPs (%d) does not match the number allocated (%d) for ClusterGlobalEgressIP %q",
			numberOfIPs, len(clusterGlobalEgressIP.Status.AllocatedIPs), clusterGlobalEgressIP.Name)

		retValue = false
	}

	condition := meta.FindStatusCondition(clusterGlobalEgressIP.Status.Conditions, string(submarinerv1.GlobalEgressIPAllocated))
	if condition == nil {
		status.Failure("ClusterGlobalEgressIP %q is missing the %q status condition", clusterGlobalEgressIP.Name,
			submarinerv1.GlobalEgressIPAllocated)

		retValue = false
	} else if condition.Status != metav1.ConditionTrue {
		status.Failure("The allocation of global IPs for ClusterGlobalEgressIP %q failed with reason %q and message %q",
			clusterGlobalEgressIP.Name, condition.Reason, condition.Message)

		retValue = false
	}

	return retValue
}

func checkGlobalEgressIps(cluster *cmd.Cluster, status reporter.Interface) bool {
	globalEgressIps, err := cluster.SubmClient.SubmarinerV1().GlobalEgressIPs(
		corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.Warning("Error obtaining GlobalEgressIPs resources: %v", err)
		return false
	}

	retValue := true

	for i := range globalEgressIps.Items {
		gip := globalEgressIps.Items[i]
		numberOfIPs := -1

		if gip.Spec.NumberOfIPs != nil {
			numberOfIPs = *gip.Spec.NumberOfIPs
		}

		if numberOfIPs != len(gip.Status.AllocatedIPs) {
			status.Failure("The number of requested IPs (%d) does not match the number allocated (%d) for GlobalEgressIP %q",
				numberOfIPs, len(gip.Status.AllocatedIPs), gip.Name)

			retValue = false
		}
	}

	return retValue
}

func checkGlobalIngressIps(cluster *cmd.Cluster, status reporter.Interface) bool {
	mcsClient, err := mcsClientset.NewForConfig(cluster.Config)
	if err != nil {
		status.Warning("Error obtaining mcs client: %v", err)
		return false
	}

	serviceExports, err := mcsClient.MulticlusterV1alpha1().ServiceExports(corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.Warning("Error listing ServiceExport resources: %v", err)
		return false
	}

	retValue := true

	for i := range serviceExports.Items {
		ns := serviceExports.Items[i].GetNamespace()
		name := serviceExports.Items[i].GetName()

		svc, err := cluster.KubeClient.CoreV1().Services(ns).Get(context.TODO(), name, metav1.GetOptions{})

		if apierrors.IsNotFound(err) {
			status.Warning("No matching Service resource found for exported service \"%s/%s\"", ns, name)
			continue
		}

		if err != nil {
			status.Failure("Error retrieving Service \"%s/%s\", %v", ns, name, err)

			retValue = false

			continue
		}

		if svc.Spec.Type != corev1.ServiceTypeClusterIP {
			continue
		}

		globalIngress, err := cluster.SubmClient.SubmarinerV1().GlobalIngressIPs(ns).Get(context.TODO(), name, metav1.GetOptions{})

		if apierrors.IsNotFound(err) {
			status.Failure("No matching GlobalIngressIP resource found for exported service \"%s/%s\"", ns, name)

			retValue = false

			continue
		}

		if err != nil {
			status.Failure("Error retrieving GlobalIngressIP for exported service \"%s/%s\": %v", ns, name, err)
			return false
		}

		if globalIngress.Status.AllocatedIP == "" {
			status.Failure("No global IP was allocated for the GlobalIngressIP associated with exported service \"%s/%s\"", ns, name)

			retValue = false

			continue
		}

		svcs, err := cluster.KubeClient.CoreV1().Services(ns).List(
			context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("submariner.io/exportedServiceRef=%s", name)})
		if err != nil {
			status.Failure("Error listing internal Services \"%s/%s\": %v", ns, name, err)

			retValue = false

			continue
		}

		if len(svcs.Items) == 0 {
			status.Failure("No internal service found for exported service \"%s/%s\"", ns, name)

			retValue = false

			continue
		}

		if len(svcs.Items) > 1 {
			status.Failure("Found %d internal services for exported service \"%s/%s\" - expected 1", len(svcs.Items), ns, name)

			retValue = false

			continue
		}

		if svcs.Items[0].Spec.ExternalIPs[0] != globalIngress.Status.AllocatedIP {
			status.Failure(
				"The external IP (%s) for internal svc associated with exported svc \"%s/%s\" doesn't match allocated IP (%s) in GlobalIngressIP %q",
				svcs.Items[0].Spec.ExternalIPs[0], ns, name, globalIngress.Status.AllocatedIP, globalIngress.Name)

			retValue = false
		}
	}

	return retValue
}
