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
	"github.com/submariner-io/submariner-operator/pkg/subctl/cmd"
	"github.com/submariner-io/submariner/pkg/globalnet/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mcsClientset "sigs.k8s.io/mcs-api/pkg/client/clientset/versioned"
)

func init() {
	diagnoseCmd.AddCommand(&cobra.Command{
		Use:   "globalnet",
		Short: "Check globalnet configuration",
		Long:  "This command checks globalnet configuration",
		Run: func(command *cobra.Command, args []string) {
			cmd.ExecuteMultiCluster(restConfigProducer, checkGlobalNet)
		},
	})
}

func checkGlobalNet(cluster *cmd.Cluster) bool {
	status := cli.NewStatus()

	if cluster.Submariner == nil {
		status.Start(cmd.SubmMissingMessage)
		status.EndWith(cli.Warning)

		return true
	}

	status.Start("Checking Globalnet configuration")

	if cluster.Submariner.Spec.GlobalCIDR != "" {
		checkClusterGlobalEgressIps(cluster, status)
		checkGlobalEgressIps(cluster, status)
		checkGlobalIngressIps(cluster, status)

		if status.HasFailureMessages() {
			status.EndWith(cli.Failure)
			return false
		}

		status.EndWithSuccess("Globalnet is enabled and properly configured")
	} else {
		status.EndWithSuccess("Globalnet is disabled")
	}

	return true
}

func checkClusterGlobalEgressIps(cluster *cmd.Cluster, status *cli.Status) {
	clusterGlobalEgress, err := cluster.SubmClient.SubmarinerV1().ClusterGlobalEgressIPs(
		corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.QueueFailureMessage(fmt.Sprintf("Error obtaining the clusterGlobalEgressIPs cr: %v", err))
		return
	}

	if len(clusterGlobalEgress.Items) != 1 {
		status.QueueFailureMessage(fmt.Sprintf("Number (%d) of clusterGlobalEgress resources != 1", len(clusterGlobalEgress.Items)))
		return
	}

	if clusterGlobalEgress.Items[0].Name != constants.ClusterGlobalEgressIPName {
		status.QueueFailureMessage(fmt.Sprintf("clusterGlobalEgress name (%s) not equals to default name", clusterGlobalEgress.Items[0].Name))
		return
	}
}

func checkGlobalEgressIps(cluster *cmd.Cluster, status *cli.Status) {
	globalEgressIps, err := cluster.SubmClient.SubmarinerV1().GlobalEgressIPs(
		corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.QueueFailureMessage(fmt.Sprintf("Error obtaining GlobalEgressIPs resources: %v", err))
		return
	}

	for i := range globalEgressIps.Items {
		if *globalEgressIps.Items[i].Spec.NumberOfIPs != len(globalEgressIps.Items[i].Status.AllocatedIPs) {
			status.QueueFailureMessage(fmt.Sprintf("Error GlobalEgressIPs(%s) NumberOfIPs != AllocatedIPs", globalEgressIps.Items[i].Name))
			return
		}
	}
}

func checkGlobalIngressIps(cluster *cmd.Cluster, status *cli.Status) {
	mcsClient, err := mcsClientset.NewForConfig(cluster.Config)
	if err != nil {
		status.QueueFailureMessage(fmt.Sprintf("Error obtaining mcs client: %v", err))
		return
	}

	serviceExports, err := mcsClient.MulticlusterV1alpha1().ServiceExports(corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		status.QueueFailureMessage(fmt.Sprintf("Error obtaining ServiceExport resources: %v", err))
		return
	}

	for i := range serviceExports.Items {
		ns := serviceExports.Items[i].GetNamespace()
		name := serviceExports.Items[i].GetName()

		svc, err := cluster.KubeClient.CoreV1().Services(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			status.QueueFailureMessage(fmt.Sprintf("Error matching Service resource for exported service(%s,%s).%v", ns, name, err))
			return
		}

		if svc.Spec.Type == corev1.ServiceTypeClusterIP {
			globalIngress, err := cluster.SubmClient.SubmarinerV1().GlobalIngressIPs(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				status.QueueFailureMessage(fmt.Sprintf("Error no GlobalIngressIP resource allocated for service(%s,%s).%v", ns, name, err))
				return
			}

			if globalIngress.Status.AllocatedIP == "" {
				status.QueueFailureMessage(fmt.Sprintf("Error GlobalIngressIP(%s,%s) - AllocatedIP is empty", ns, name))
				return
			}

			svcs, err := cluster.KubeClient.CoreV1().Services(ns).List(
				context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("submariner.io/exportedServiceRef=%s", name)})
			if err != nil {
				status.QueueFailureMessage(fmt.Sprintf("Error finding internal service for exported service(%s,%s).%v", ns, name, err))
				return
			}

			if len(svcs.Items) != 1 {
				status.QueueFailureMessage(fmt.Sprintf("Error finding internal service for exported service(%s,%s).%v", ns, name, err))
				return
			}

			if svcs.Items[0].Spec.ExternalIPs[0] != globalIngress.Status.AllocatedIP {
				status.QueueFailureMessage(fmt.Sprintf("GlobalIngressIP != ExternalIP of internal Service for exported service(%s,%s)", ns, name))
				return
			}
		}
	}
}
