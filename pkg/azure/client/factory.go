// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// AzureFactoryOption represents an option for the AzureFactory constructor.
type AzureFactoryOption func(*azureFactory)

// WithCloudConfiguration is the option that sets the cloud configuration on the factory
func WithCloudConfiguration(cloudConfiguration cloud.Configuration) AzureFactoryOption {
	return func(f *azureFactory) {
		f.clientOpts.Cloud = cloudConfiguration
	}
}

// AzureFactory is an implementation of Factory to produce clients for various Azure services.
type azureFactory struct {
	auth            *internal.ClientAuth
	tokenCredential azcore.TokenCredential
	clientOpts      *policy.ClientOptions
}

// NewAzureClientFactoryFromSecret builds the factory from the given secret (by ref).
func NewAzureClientFactoryFromSecret(
	ctx context.Context,
	client client.Client,
	secretRef corev1.SecretReference,
	isDNSSecret bool,
	options ...AzureFactoryOption,
) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, isDNSSecret)
	if err != nil {
		return nil, err
	}
	return NewAzureClientFactory(auth, options...)
}

// NewAzureClientFactory constructs a new factory using the provided Credentials and applying the provided options.
func NewAzureClientFactory(authCredentials *internal.ClientAuth, options ...AzureFactoryOption) (Factory, error) {
	// prepare tokenCredential for more convenient access later on
	cred, err := authCredentials.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}

	factory := &azureFactory{
		auth:            authCredentials,
		tokenCredential: cred,
		clientOpts:      DefaultAzureClientOpts(),
	}

	for _, option := range options {
		option(factory)
	}

	return *factory, nil
}

// StorageAccount returns an Azure storage account client.
func (f azureFactory) StorageAccount() (StorageAccount, error) {
	return NewStorageAccountClient(f.auth, f.tokenCredential, f.clientOpts)
}

// DNSZone returns an Azure DNS zone client.
func (f azureFactory) DNSZone() (DNSZone, error) {
	return NewDnsZoneClient(f.auth, f.tokenCredential, f.clientOpts)
}

// DNSRecordSet returns an Azure DNS record set client.
func (f azureFactory) DNSRecordSet() (DNSRecordSet, error) {
	return NewDnsRecordSetClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Group returns an Azure resource group client.
func (f azureFactory) Group() (ResourceGroup, error) {
	return NewResourceGroupsClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Resource returns an Azure resource client.
func (f azureFactory) Resource() (Resource, error) {
	return NewResourceClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Vmss returns an Azure virtual machine scale set client.
func (f azureFactory) Vmss() (Vmss, error) {
	return NewVmssClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// VirtualMachine returns an Azure virtual machine client.
func (f azureFactory) VirtualMachine() (VirtualMachine, error) {
	return NewVMClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NetworkSecurityGroup returns an Azure network security group client.
func (f azureFactory) NetworkSecurityGroup() (NetworkSecurityGroup, error) {
	return NewSecurityGroupClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// PublicIP returns an Azure network PublicIPClient.
func (f azureFactory) PublicIP() (PublicIP, error) {
	return NewPublicIPClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NetworkInterface returns an Azure network interface client.
func (f azureFactory) NetworkInterface() (NetworkInterface, error) {
	return NewNetworkInterfaceClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Disk returns an Azure disk client.
func (f azureFactory) Disk() (Disk, error) {
	return NewDisksClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Vnet returns an Azure Vnet client.
func (f azureFactory) Vnet() (VirtualNetwork, error) {
	return NewVnetClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Subnet returns an Azure Subnet client.
func (f azureFactory) Subnet() (Subnet, error) {
	return NewSubnetsClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// RouteTables returns an Azure RouteTables client.
func (f azureFactory) RouteTables() (RouteTables, error) {
	return NewRouteTablesClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NatGateway returns a NatGateway client.
func (f azureFactory) NatGateway() (NatGateway, error) {
	return NewNatGatewaysClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// AvailabilitySet returns an AvailabilitySet client.
func (f azureFactory) AvailabilitySet() (AvailabilitySet, error) {
	return NewAvailabilitySetClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// ManagedUserIdentity returns a ManagedUserIdentity client.
func (f azureFactory) ManagedUserIdentity() (ManagedUserIdentity, error) {
	return NewManagedUserIdentityClient(f.auth, f.tokenCredential, f.clientOpts)
}

// VirtualMachineImages returns a VirtualMachineImages client.
func (f azureFactory) VirtualMachineImages() (VirtualMachineImages, error) {
	return NewVirtualMachineImagesClient(f.auth, f.tokenCredential, f.clientOpts)
}

// NewBlobStorageClient reads the secret from the passed reference and return an Azure (blob) storage client.
func NewBlobStorageClient(ctx context.Context, c client.Client, secretRef corev1.SecretReference, cloudConfiguration *azure.CloudConfiguration) (BlobStorage, error) {
	var storageDomain string

	// Unfortunately the valid values for storage domains run by Microsoft do not seem to be part of any sdk module. They might be queryable from the cloud configuration,
	// but I also haven't been able to find a documented list of proper ServiceName values.
	// Furthermore, it seems there is still no unified way of specifying the cloud instance to connect to as the domain remains part of the storage account URL while
	// the new options _also_ allow configuring the cloud instance.
	// For the time being (until further testing can be done) I assume that the instance that is pointed at by the URL wins so let's keep the old logic for building the
	// service URL.
	if cloudConfiguration == nil {
		storageDomain = "blob.core.windows.net"
	} else {
		cloudConfigurationName := cloudConfiguration.Name
		switch {
		case strings.EqualFold(cloudConfigurationName, "AzurePublic"):
			storageDomain = "blob.core.windows.net"
		case strings.EqualFold(cloudConfigurationName, "AzureGovernment"):
			// Note: This differs from the one mentioned in the docs ("blob.core.govcloudapi.net") but should be the right one.
			// ref.: https://github.com/google/go-cloud/blob/be1b4aee38955e1b8cd1c46f8f47fb6f9d820a9b/blob/azureblob/azureblob.go#L162
			storageDomain = "blob.core.usgovcloudapi.net"
		case strings.EqualFold(cloudConfigurationName, "AzureChina"):
			// This is an educated guess
			storageDomain = "blob.core.chinacloudapi.cn"

		default:
			return nil, fmt.Errorf("unknown cloud configuration name '%s'", cloudConfigurationName)
		}
	}

	blobStorageClient, err := newStorageClient(ctx, c, &secretRef, storageDomain)
	if err != nil {
		return nil, err
	}

	return blobStorageClient, nil
}
