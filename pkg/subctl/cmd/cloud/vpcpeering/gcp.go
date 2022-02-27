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

package vpcpeering

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	pre "github.com/submariner-io/cloud-prepare/pkg/gcp"
	gcpClientIface "github.com/submariner-io/cloud-prepare/pkg/gcp/client"
	"github.com/submariner-io/submariner-operator/internal/exit"
	"github.com/submariner-io/submariner-operator/pkg/subctl/cmd/cloud/gcp"
	cloudutils "github.com/submariner-io/submariner-operator/pkg/subctl/cmd/cloud/utils"
	"github.com/submariner-io/submariner-operator/pkg/subctl/cmd/utils"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

const (
	targetInfraIDFlag         = "target-infra-id"
	targetRegionFlag          = "target-region"
	targetProjectIDFlag       = "target-project-id"
	targetOcpMetadataFileFlag = "target-ocp-metadata"
	targetCredentialsFlag     = "target-credentials"
)

var (
	targetInfraID         string
	targetRegion          string
	targetProjectID       string
	targetOcpMetadataFile string
	targetCredentialsFile string
)

// NewCommand returns a new cobra.Command used to create a VPC Peering on a cloud infrastructure.
func newGCPVPCPeeringCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcp",
		Short: "Create a VPC Peering on GCP cloud",
		Long:  "This command prepares an OpenShift installer-provisioned infrastructure (IPI) on GCP cloud for Submariner installation.",
		Run:   vpcPeerGcp,
	}

	gcp.AddGCPFlags(cmd)
	cmd.Flags().StringVar(&targetInfraID, targetInfraIDFlag, "", "GCP infra ID of target")
	cmd.Flags().StringVar(&targetRegion, targetRegionFlag, "", "GCP region of target")
	cmd.Flags().StringVar(&targetProjectID, targetProjectIDFlag, "", "GCP project ID of target")
	cmd.Flags().StringVar(&targetOcpMetadataFile, targetOcpMetadataFileFlag, "",
		"OCP metadata.json file of target (or the directory containing it) from which to read the GCP infra ID "+
			"and region from (takes precedence over the specific flags)")

	dirname, err := os.UserHomeDir()
	if err != nil {
		exit.OnErrorWithMessage(err, "failed to find home directory")
	}

	defaultCredentials := filepath.FromSlash(fmt.Sprintf("%s/.gcp/osServiceAccount.json", dirname))
	cmd.Flags().StringVar(&targetCredentialsFile, targetCredentialsFlag, defaultCredentials, "GCP credentials configuration file of target")
	return cmd
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
		InfraID string `json:"infraID"`
		GCP     struct {
			Region    string `json:"region"`
			ProjectID string `json:"projectID"`
		} `json:"gcp"`
	}

	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling data")
	}

	targetInfraID = metadata.InfraID
	targetRegion = metadata.GCP.Region
	targetProjectID = metadata.GCP.ProjectID

	return nil
}

func validatePeeringFlags() {
	if targetOcpMetadataFile != "" {
		err := initializeFlagsFromOCPMetadata(targetOcpMetadataFile)
		exit.OnErrorWithMessage(err, "Failed to read GCP Cluster information from OCP metadata file")
	} else {
		utils.ExpectFlag(targetInfraIDFlag, targetInfraID)
		utils.ExpectFlag(targetRegionFlag, targetRegion)
		utils.ExpectFlag(targetProjectIDFlag, targetProjectID)
	}
}

func getGCPCredentials() (*google.Credentials, error) {
	authJSON, err := os.ReadFile(targetCredentialsFile)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading file %q", targetCredentialsFile)
	}

	creds, err := google.CredentialsFromJSON(context.TODO(), authJSON, dns.CloudPlatformScope)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing credentials file")
	}

	return creds, nil
}

func vpcPeerGcp(cmd *cobra.Command, args []string) {
	validatePeeringFlags()
	reporter := cloudutils.NewStatusReporter()

	//Create credentials for GCP client options
	reporter.Started("Retrieving target GCP credentials from your GCP configuration")
	creds, err := getGCPCredentials()
	exit.OnErrorWithMessage(err, "Failed to get target GCP credentials")
	reporter.Succeeded("")

	options := []option.ClientOption{
		option.WithCredentials(creds),
		option.WithUserAgent("open-cluster-management.io submarineraddon/v1"),
	}

	gcpClient, err := gcpClientIface.NewClient(targetProjectID, options)
	exit.OnErrorWithMessage(err, "Failed to initialize a GCP Client")

	reporter.Started("Initializing GCP connectivity")

	gcpCloudInfo := pre.CloudInfo{
		ProjectID: targetProjectID,
		InfraID:   targetInfraID,
		Region:    targetRegion,
		Client:    gcpClient,
	}

	targetCloud := cloudpreparegcp.NewCloud(gcpCloudInfo)

	// reporter.Succeeded("")
	err1 := gcp.RunOnGCP(*parentRestConfigProducer, "", false,
		func(cloud api.Cloud, gwDeployer api.GatewayDeployer, reporter api.Reporter) error {
			return cloud.CreateVpcPeering(targetCloud, reporter)
		})
	if err1 != nil {
		exit.OnErrorWithMessage(err, "Failed to create VPC Peering on GCP cloud")
	}
}
