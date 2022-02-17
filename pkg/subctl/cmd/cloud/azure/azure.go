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

package azure

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/submariner-operator/internal/exit"
	"github.com/submariner-io/submariner-operator/internal/restconfig"
	cloudutils "github.com/submariner-io/submariner-operator/pkg/subctl/cmd/cloud/utils"
	"github.com/submariner-io/submariner-operator/pkg/subctl/cmd/utils"

	"os"
	"path/filepath"

	"github.com/submariner-io/cloud-prepare/pkg/azure"
	"k8s.io/client-go/kubernetes"
)

var (
	subscriptionID  string
	infraID         string
	region          string
	ocpMetadataFile string
	authFile        string
	baseGroupName   string
	authorizer      autorest.Authorizer
)

const (
	infraIDFlag        = "infra-id"
	regionFlag         = "region"
)

// AddAzureFlags adds basic flags needed by Azure.
func AddAzureFlags(command *cobra.Command) {
	command.Flags().StringVar(&infraID, infraIDFlag, "", "Azure infra ID")
	command.Flags().StringVar(&region, regionFlag, "", "Azure region")
	command.Flags().StringVar(&ocpMetadataFile, "ocp-metadata", "",
		"OCP metadata.json file (or directory containing it) to read Azure infra ID and region from (Takes precedence over the flags)")
	command.Flags().StringVar(&authFile, "auth-file", "", "Azure authorization file to be used")
	command.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID")
}

// RunOnAzure runs the given function on Azure, supplying it with a cloud instance connected to Azure and a reporter that writes to CLI.
// The function makes sure that infraID and region are specified, and extracts the credentials from a secret in order to connect to Azure.
func RunOnAzure(restConfigProducer restconfig.Producer, function func(cloud api.Cloud, gwDeployer api.GatewayDeployer,
	reporter api.Reporter) error) error {
	if ocpMetadataFile != "" {
		err := initializeFlagsFromOCPMetadata(ocpMetadataFile)
		exit.OnErrorWithMessage(err, "Failed to read Azure information from OCP metadata file")
	} else {
		utils.ExpectFlag(infraIDFlag, infraID)
		utils.ExpectFlag(regionFlag, region)
	}

	utils.ExpectFlag("auth-file", authFile)
	err := os.Setenv("AZURE_AUTH_LOCATION", authFile)
	exit.OnErrorWithMessage(err, "Error locating authorization file")

	reporter := cloudutils.NewStatusReporter()
	reporter.Started("Retrieving Azure credentials from your Azure configuration")

	authorizer, err = auth.NewAuthorizerFromCLI()
	exit.OnErrorWithMessage(err, "Error getting an authorizer for Azure")

	reporter.Succeeded("")

	k8sConfig, err := restConfigProducer.ForCluster()
	exit.OnErrorWithMessage(err, "Failed to initialize a Kubernetes config")

	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	exit.OnErrorWithMessage(err,"Failed to create Kubernetes client")

	k8sClientSet := k8s.NewInterface(clientSet)

	cloudInfo := azure.CloudInfo{
		SubscriptionID: subscriptionID,
		InfraID:        infraID,
		Region:         region,
		BaseGroupName:  baseGroupName,
		Authorizer:     authorizer,
		K8sClient:      k8sClientSet,
    }

	azureCloud := azure.NewCloud(&cloudInfo)

	gwDeployer, err := azure.NewOcpGatewayDeployer(azureCloud) // TODO: Edit this once gateway deployer is implemented
	exit.OnErrorWithMessage(err, "Failed to initialize a GatewayDeployer config")

	//return function(azureCloud, gwDeployer, reporter)
}

func initializeFlagsFromOCPMetadata(metadataFile string) error {
	fileInfo, err := os.Stat(metadataFile)
	if err != nil {
		return errors.Wrapf(err, "failed to stat file %q", metadataFile)
	}

	if fileInfo.IsDir() {
		metadataFile = filepath.Join(metadataFile, "metadata.json")
	}

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return errors.Wrapf(err, "error reading file %q", metadataFile)
	}

	var metadata struct {
		InfraID   string `json:"infraID"`
		Azure     struct {
			Region            string `json:"region"`
			ResourceGroupName string `json:"resourceGroupName"`
		} `json:"azure"`
	}

	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling data")
	}

	infraID = metadata.InfraID
	region = metadata.Azure.Region
	if metadata.Azure.ResourceGroupName != "" {
		baseGroupName = metadata.Azure.ResourceGroupName
	} else {
		baseGroupName = infraID + "-rg"
	}

	return nil
}
