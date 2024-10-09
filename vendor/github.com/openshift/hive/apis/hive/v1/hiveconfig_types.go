package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/hive/apis/hive/v1/azure"
	"github.com/openshift/hive/apis/hive/v1/metricsconfig"
)

// HiveConfigSpec defines the desired state of Hive
type HiveConfigSpec struct {

	// TargetNamespace is the namespace where the core Hive components should be run. Defaults to "hive". Will be
	// created if it does not already exist. All resource references in HiveConfig can be assumed to be in the
	// TargetNamespace.
	// NOTE: Whereas it is possible to edit this value, causing hive to "move" its core components to the new
	// namespace, the old namespace is not deleted, as it will still contain resources created by kubernetes
	// and/or other OpenShift controllers.
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// ManagedDomains is the list of DNS domains that are managed by the Hive cluster
	// When specifying 'manageDNS: true' in a ClusterDeployment, the ClusterDeployment's
	// baseDomain should be a direct child of one of these domains, otherwise the
	// ClusterDeployment creation will result in a validation error.
	// +optional
	ManagedDomains []ManageDNSConfig `json:"managedDomains,omitempty"`

	// AdditionalCertificateAuthoritiesSecretRef is a list of references to secrets in the
	// TargetNamespace that contain an additional Certificate Authority to use when communicating
	// with target clusters. These certificate authorities will be used in addition to any self-signed
	// CA generated by each cluster on installation. The cert data should be stored in the Secret key named 'ca.crt'.
	// +optional
	AdditionalCertificateAuthoritiesSecretRef []corev1.LocalObjectReference `json:"additionalCertificateAuthoritiesSecretRef,omitempty"`

	// GlobalPullSecretRef is used to specify a pull secret that will be used globally by all of the cluster deployments.
	// For each cluster deployment, the contents of GlobalPullSecret will be merged with the specific pull secret for
	// a cluster deployment(if specified), with precedence given to the contents of the pull secret for the cluster deployment.
	// The global pull secret is assumed to be in the TargetNamespace.
	// +optional
	GlobalPullSecretRef *corev1.LocalObjectReference `json:"globalPullSecretRef,omitempty"`

	// Backup specifies configuration for backup integration.
	// If absent, backup integration will be disabled.
	// +optional
	Backup BackupConfig `json:"backup,omitempty"`

	// FailedProvisionConfig is used to configure settings related to handling provision failures.
	// +optional
	FailedProvisionConfig FailedProvisionConfig `json:"failedProvisionConfig,omitempty"`

	// ServiceProviderCredentialsConfig is used to configure credentials related to being a service provider on
	// various cloud platforms.
	// +optional
	ServiceProviderCredentialsConfig ServiceProviderCredentials `json:"serviceProviderCredentialsConfig,omitempty"`

	// LogLevel is the level of logging to use for the Hive controllers.
	// Acceptable levels, from coarsest to finest, are panic, fatal, error, warn, info, debug, and trace.
	// The default level is info.
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// SyncSetReapplyInterval is a string duration indicating how much time must pass before SyncSet resources
	// will be reapplied.
	// The default reapply interval is two hours.
	SyncSetReapplyInterval string `json:"syncSetReapplyInterval,omitempty"`

	// MachinePoolPollInterval is a string duration indicating how much time must pass before checking whether
	// remote resources related to MachinePools need to be reapplied. Set to zero to disable polling -- we'll
	// only reconcile when hub objects change.
	// The default interval is 30m.
	MachinePoolPollInterval string `json:"machinePoolPollInterval,omitempty"`

	// ClusterVersionPollInterval is a string duration indicating how much time must pass before checking
	// whether we need to update the hive.openshift.io/version* labels on ClusterDeployment. If zero or unset,
	// we'll only reconcile when the ClusterDeployment changes.
	ClusterVersionPollInterval string `json:"clusterVersionPollInterval,omitempty"`

	// MaintenanceMode can be set to true to disable the hive controllers in situations where we need to ensure
	// nothing is running that will add or act upon finalizers on Hive types. This should rarely be needed.
	// Sets replicas to 0 for the hive-controllers deployment to accomplish this.
	MaintenanceMode *bool `json:"maintenanceMode,omitempty"`

	// DeprovisionsDisabled can be set to true to block deprovision jobs from running.
	DeprovisionsDisabled *bool `json:"deprovisionsDisabled,omitempty"`

	// DeleteProtection can be set to "enabled" to turn on automatic delete protection for ClusterDeployments. When
	// enabled, Hive will add the "hive.openshift.io/protected-delete" annotation to new ClusterDeployments. Once a
	// ClusterDeployment has been installed, a user must remove the annotation from a ClusterDeployment prior to
	// deleting it.
	// +kubebuilder:validation:Enum=enabled
	// +optional
	DeleteProtection DeleteProtectionType `json:"deleteProtection,omitempty"`

	// DisabledControllers allows selectively disabling Hive controllers by name.
	// The name of an individual controller matches the name of the controller as seen in the Hive logging output.
	DisabledControllers []string `json:"disabledControllers,omitempty"`

	// ControllersConfig is used to configure different hive controllers
	// +optional
	ControllersConfig *ControllersConfig `json:"controllersConfig,omitempty"`

	// DeploymentConfig is used to configure (pods/containers of) the Deployments generated by hive-operator.
	// +optional
	DeploymentConfig *[]DeploymentConfig `json:"deploymentConfig,omitempty"`

	// PrivateLink is used to configure the privatelink controller.
	// +optional
	PrivateLink *PrivateLinkConfig `json:"privateLink,omitempty"`

	// AWSPrivateLink defines the configuration for the aws-private-link controller.
	// It provides 3 major pieces of information required by the controller,
	// 1. The Credentials that should be used to create AWS PrivateLink resources other than
	//     what exist in the customer's account.
	// 2. A list of VPCs that can be used by the controller to choose one to create AWS VPC Endpoints
	//     for the AWS VPC Endpoint Services created for ClusterDeployments in their
	//     corresponding regions.
	// 3. A list of VPCs that should be able to resolve the DNS addresses setup for Private Link.
	AWSPrivateLink *AWSPrivateLinkConfig `json:"awsPrivateLink,omitempty"`

	// ReleaseImageVerificationConfigMapRef is a reference to the ConfigMap that
	// will be used to verify release images.
	//
	// The config map structure is exactly the same as the config map used for verification of release
	// images for OpenShift 4 during upgrades. Therefore you can usually set this to the config map shipped
	// as part of OpenShift (openshift-config-managed/release-verification).
	//
	// See https://github.com/openshift/cluster-update-keys for more details.
	// The keys within the config map in the data field define how verification is performed:
	//
	// verifier-public-key-*: One or more GPG public keys in ASCII form that must have signed the
	//                        release image by digest.
	//
	// store-*: A URL (scheme file://, http://, or https://) location that contains signatures. These
	//          signatures are in the atomic container signature format. The URL will have the digest
	//          of the image appended to it as "<STORE>/<ALGO>=<DIGEST>/signature-<NUMBER>" as described
	//          in the container image signing format. The docker-image-manifest section of the
	//          signature must match the release image digest. Signatures are searched starting at
	//          NUMBER 1 and incrementing if the signature exists but is not valid. The signature is a
	//          GPG signed and encrypted JSON message. The file store is provided for testing only at
	//          the current time, although future versions of the CVO might allow host mounting of
	//          signatures.
	//
	// See https://github.com/containers/image/blob/ab49b0a48428c623a8f03b41b9083d48966b34a9/docs/signature-protocols.md
	// for a description of the signature store
	//
	// The returned verifier will require that any new release image will only be considered verified
	// if each provided public key has signed the release image digest. The signature may be in any
	// store and the lookup order is internally defined.
	//
	// If not set, no verification will be performed.
	// +optional
	ReleaseImageVerificationConfigMapRef *ReleaseImageVerificationConfigMapReference `json:"releaseImageVerificationConfigMapRef,omitempty"`
	// ArgoCD specifies configuration for ArgoCD integration. If enabled, Hive will automatically add provisioned
	// clusters to ArgoCD, and remove them when they are deprovisioned.
	ArgoCD ArgoCDConfig `json:"argoCDConfig,omitempty"`

	FeatureGates *FeatureGateSelection `json:"featureGates,omitempty"`

	// ExportMetrics has been disabled and has no effect. If upgrading from a version where it was
	// active, please be aware of the following in your HiveConfig.Spec.TargetNamespace (default
	// `hive` if unset):
	// 1) ServiceMonitors named hive-controllers and hive-clustersync;
	// 2) Role and RoleBinding named prometheus-k8s;
	// 3) The `openshift.io/cluster-monitoring` metadata.label on the Namespace itself.
	// You may wish to delete these resources. Or you may wish to continue using them to enable
	// monitoring in your environment; but be aware that hive will no longer reconcile them.
	ExportMetrics bool `json:"exportMetrics,omitempty"`

	// MetricsConfig encapsulates metrics specific configurations, like opting in for certain metrics.
	// +optional
	MetricsConfig *metricsconfig.MetricsConfig `json:"metricsConfig,omitempty"`
}

// ReleaseImageVerificationConfigMapReference is a reference to the ConfigMap that
// will be used to verify release images.
type ReleaseImageVerificationConfigMapReference struct {
	// Namespace of the ConfigMap
	Namespace string `json:"namespace"`
	// Name of the ConfigMap
	Name string `json:"name"`
}

// PrivateLinkConfig defines the configuration for the privatelink controller.
type PrivateLinkConfig struct {
	// GCP is the configuration for GCP hub and link resources.
	// +optional
	GCP *GCPPrivateServiceConnectConfig `json:"gcp,omitempty"`
}

// AWSPrivateLinkConfig defines the configuration for the aws-private-link controller.
type AWSPrivateLinkConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// AWS for creating the resources for AWS PrivateLink.
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// EndpointVPCInventory is a list of VPCs and the corresponding subnets in various AWS regions.
	// The controller uses this list to choose a VPC for creating AWS VPC Endpoints. Since the
	// VPC Endpoints must be in the same region as the ClusterDeployment, we must have VPCs in that
	// region to be able to setup Private Link.
	EndpointVPCInventory []AWSPrivateLinkInventory `json:"endpointVPCInventory,omitempty"`

	// AssociatedVPCs is the list of VPCs that should be able to resolve the DNS addresses
	// setup for Private Link. This allows clients in VPC to resolve the AWS PrivateLink address
	// using AWS's default DNS resolver for Private Route53 Hosted Zones.
	//
	// This list should at minimum include the VPC where the current Hive controller is running.
	AssociatedVPCs []AWSAssociatedVPC `json:"associatedVPCs,omitempty"`

	// DNSRecordType defines what type of DNS record should be created in Private Hosted Zone
	// for the customer cluster's API endpoint (which is the VPC Endpoint's regional DNS name).
	//
	// +kubebuilder:default=Alias
	// +optional
	DNSRecordType AWSPrivateLinkDNSRecordType `json:"dnsRecordType,omitempty"`
}

// AWSPrivateLinkDNSRecordType defines what type of DNS record should be created in Private Hosted Zone
// for the customer cluster's API endpoint (which is the VPC Endpoint's regional DNS name).
// +kubebuilder:validation:Enum=Alias;ARecord
type AWSPrivateLinkDNSRecordType string

const (
	// AliasAWSPrivateLinkDNSRecordType uses Route53 Alias record type for pointing the customer cluster's
	// API DNS name to the DNS name of the VPC endpoint. This is the default and should be used for most
	// cases as it is provided at no extra cost in terms of DNS queries and usually resolves faster in AWS
	// environments.
	AliasAWSPrivateLinkDNSRecordType AWSPrivateLinkDNSRecordType = "Alias"

	// ARecordAWSPrivateLinkDNSRecordType uses Route53 A record type for pointing the customer cluster's
	// API DNS name to the DNS name of the VPC endpoint. This should be used when Alias record type cannot
	// be used or other restrictions prevent use of Alias records.
	ARecordAWSPrivateLinkDNSRecordType AWSPrivateLinkDNSRecordType = "ARecord"
)

// AWSPrivateLinkInventory is a VPC and its corresponding subnets in an AWS region.
// This VPC will be used to create an AWS VPC Endpoint whenever there is a VPC Endpoint Service
// created for a ClusterDeployment.
type AWSPrivateLinkInventory struct {
	AWSPrivateLinkVPC `json:",inline"`
	Subnets           []AWSPrivateLinkSubnet `json:"subnets"`
}

// AWSAssociatedVPC defines a VPC that should be able to resolve the DNS addresses
// setup for Private Link.
type AWSAssociatedVPC struct {
	AWSPrivateLinkVPC `json:",inline"`
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// AWS for associating the VPC with the Private HostedZone created for PrivateLink.
	// When not provided, the common credentials for the controller should be used.
	//
	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef"`
}

// AWSPrivateLinkVPC defines an AWS VPC in a region.
type AWSPrivateLinkVPC struct {
	VPCID  string `json:"vpcID"`
	Region string `json:"region"`
}

// AWSPrivateLinkSubnet defines a subnet in the an AWS VPC.
type AWSPrivateLinkSubnet struct {
	SubnetID         string `json:"subnetID"`
	AvailabilityZone string `json:"availabilityZone"`
}

// ServiceProviderCredentials is used to configure credentials related to being a service provider on
// various cloud platforms.
type ServiceProviderCredentials struct {
	// AWS is used to configure credentials related to being a service provider on AWS.
	// +optional
	AWS *AWSServiceProviderCredentials `json:"aws,omitempty"`
}

// AWSServiceProviderCredentials is used to configure credentials related to being a service
// provider on AWS.
type AWSServiceProviderCredentials struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// AWS to become the Service Provider. Being a Service Provider allows the controllers
	// to assume the role in customer AWS accounts to manager clusters.
	// +optional
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`
}

// GCPPrivateServiceConnectConfig defines the gcp private service connect config for the private-link controller.
type GCPPrivateServiceConnectConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// GCP for creating the resources for GCP Private Service Connect
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// EndpointVPCInventory is a list of VPCs and the corresponding subnets in various GCP regions.
	// The controller uses this list to choose a VPC for creating GCP Endpoints. Since the VPC Endpoints
	// must be in the same region as the ClusterDeployment, we must have VPCs in that region to be able
	// to setup Private Service Connect.
	// +optional
	EndpointVPCInventory []GCPPrivateServiceConnectInventory `json:"endpointVPCInventory,omitempty"`
}

// GCPPrivateServiceConnectInventory is a VPC and its corresponding subnets.
// This VPC will be used to create a GCP Endpoint whenever there is a Private Service Connect
// service created for a ClusterDeployment.
type GCPPrivateServiceConnectInventory struct {
	Network string                           `json:"network"`
	Subnets []GCPPrivateServiceConnectSubnet `json:"subnets"`
}

// GCPPrivateServiceConnectSubnet defines subnet and the corresponding GCP region.
type GCPPrivateServiceConnectSubnet struct {
	Subnet string `json:"subnet"`
	Region string `json:"region"`
}

// FeatureSet defines the set of feature gates that should be used.
// +kubebuilder:validation:Enum="";Custom
type FeatureSet string

var (
	// DefaultFeatureSet feature set is the default things supported as part of normal supported platform.
	DefaultFeatureSet FeatureSet = ""

	// CustomFeatureSet allows the enabling or disabling of any feature. Turning this feature set on IS NOT SUPPORTED.
	// Because of its nature, this setting cannot be validated.  If you have any typos or accidentally apply invalid combinations
	// it might leave object in a state that is unrecoverable.
	CustomFeatureSet FeatureSet = "Custom"
)

// FeatureGateSelection allows selecting feature gates for the controller.
type FeatureGateSelection struct {
	// featureSet changes the list of features in the cluster.  The default is empty.  Be very careful adjusting this setting.
	// +unionDiscriminator
	// +optional
	FeatureSet FeatureSet `json:"featureSet,omitempty"`

	// custom allows the enabling or disabling of any feature.
	// Because of its nature, this setting cannot be validated.  If you have any typos or accidentally apply invalid combinations
	// might cause unknown behavior. featureSet must equal "Custom" must be set to use this field.
	// +optional
	// +nullable
	Custom *FeatureGatesEnabled `json:"custom,omitempty"`
}

// FeatureGatesEnabled is list of feature gates that must be enabled.
type FeatureGatesEnabled struct {
	// enabled is a list of all feature gates that you want to force on
	// +optional
	Enabled []string `json:"enabled,omitempty"`
}

// FeatureSets Contains a map of Feature names to Enabled/Disabled Feature.
var FeatureSets = map[FeatureSet]*FeatureGatesEnabled{
	DefaultFeatureSet: {
		Enabled: []string{},
	},
	CustomFeatureSet: {
		Enabled: []string{},
	},
}

// HiveConfigStatus defines the observed state of Hive
type HiveConfigStatus struct {
	// AggregatorClientCAHash keeps an md5 hash of the aggregator client CA
	// configmap data from the openshift-config-managed namespace. When the configmap changes,
	// admission is redeployed.
	AggregatorClientCAHash string `json:"aggregatorClientCAHash,omitempty"`

	// ObservedGeneration will record the most recently processed HiveConfig object's generation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigApplied will be set by the hive operator to indicate whether or not the LastGenerationObserved
	// was successfully reconciled.
	ConfigApplied bool `json:"configApplied,omitempty"`

	// Conditions includes more detailed status for the HiveConfig
	// +optional
	Conditions []HiveConfigCondition `json:"conditions,omitempty"`
}

// HiveConfigCondition contains details for the current condition of a HiveConfig
type HiveConfigCondition struct {
	// Type is the type of the condition.
	Type HiveConfigConditionType `json:"type"`
	// Status is the status of the condition.
	Status corev1.ConditionStatus `json:"status"`
	// LastProbeTime is the last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// HiveConfigConditionType is a valid value for HiveConfigCondition.Type
type HiveConfigConditionType string

const (
	// HiveReadyCondition is set when hive is deployed successfully and ready to provision clusters
	HiveReadyCondition HiveConfigConditionType = "Ready"
)

// ArgoCDConfig contains settings for integration with ArgoCD.
type ArgoCDConfig struct {
	// Enabled dictates if ArgoCD gitops integration is enabled.
	// If not specified, the default is disabled.
	Enabled bool `json:"enabled"`

	// Namespace specifies the namespace where ArgoCD is installed. Used for the location of cluster secrets.
	// Defaults to "argocd"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// BackupConfig contains settings for the Velero backup integration.
type BackupConfig struct {
	// Velero specifies configuration for the Velero backup integration.
	// +optional
	Velero VeleroBackupConfig `json:"velero,omitempty"`

	// MinBackupPeriodSeconds specifies that a minimum of MinBackupPeriodSeconds will occur in between each backup.
	// This is used to rate limit backups. This potentially batches together multiple changes into 1 backup.
	// No backups will be lost as changes that happen during this interval are queued up and will result in a
	// backup happening once the interval has been completed.
	// +optional
	MinBackupPeriodSeconds *int `json:"minBackupPeriodSeconds,omitempty"`
}

// VeleroBackupConfig contains settings for the Velero backup integration.
type VeleroBackupConfig struct {
	// Enabled dictates if Velero backup integration is enabled.
	// If not specified, the default is disabled.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Namespace specifies in which namespace velero backup objects should be created.
	// If not specified, the default is a namespace named "velero".
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// FailedProvisionConfig contains settings to control behavior undertaken by Hive when an installation attempt fails.
type FailedProvisionConfig struct {

	// TODO: Figure out how to mark SkipGatherLogs as deprecated (more than just a comment)

	// DEPRECATED: This flag is no longer respected and will be removed in the future.
	SkipGatherLogs bool                      `json:"skipGatherLogs,omitempty"`
	AWS            *FailedProvisionAWSConfig `json:"aws,omitempty"`
	// RetryReasons is a list of installFailingReason strings from the [additional-]install-log-regexes ConfigMaps.
	// If specified, Hive will only retry a failed installation if it results in one of the listed reasons. If
	// omitted (not the same thing as empty!), Hive will retry regardless of the failure reason. (The total number
	// of install attempts is still constrained by ClusterDeployment.Spec.InstallAttemptsLimit.)
	RetryReasons *[]string `json:"retryReasons,omitempty"`
}

// ManageDNSConfig contains the domain being managed, and the cloud-specific
// details for accessing/managing the domain.
type ManageDNSConfig struct {

	// Domains is the list of domains that hive will be managing entries for with the provided credentials.
	Domains []string `json:"domains"`

	// AWS contains AWS-specific settings for external DNS
	// +optional
	AWS *ManageDNSAWSConfig `json:"aws,omitempty"`

	// GCP contains GCP-specific settings for external DNS
	// +optional
	GCP *ManageDNSGCPConfig `json:"gcp,omitempty"`

	// Azure contains Azure-specific settings for external DNS
	// +optional
	Azure *ManageDNSAzureConfig `json:"azure,omitempty"`

	// As other cloud providers are supported, additional fields will be
	// added for each of those cloud providers. Only a single cloud provider
	// may be configured at a time.
}

// FailedProvisionAWSConfig contains AWS-specific info to upload log files.
type FailedProvisionAWSConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// AWS S3. It will need permission to upload logs to S3.
	// Secret should have keys named aws_access_key_id and aws_secret_access_key that contain the AWS credentials.
	// Example Secret:
	//   data:
	//     aws_access_key_id: minio
	//     aws_secret_access_key: minio123
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// Region is the AWS region to use for S3 operations.
	// This defaults to us-east-1.
	// For AWS China, use cn-northwest-1.
	// +optional
	Region string `json:"region,omitempty"`

	// ServiceEndpoint is the url to connect to an S3 compatible provider.
	ServiceEndpoint string `json:"serviceEndpoint,omitempty"`

	// Bucket is the S3 bucket to store the logs in.
	Bucket string `json:"bucket,omitempty"`
}

// ManageDNSAWSConfig contains AWS-specific info to manage a given domain.
type ManageDNSAWSConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// AWS Route53. It will need permission to manage entries for the domain
	// listed in the parent ManageDNSConfig object.
	// Secret should have AWS keys named 'aws_access_key_id' and 'aws_secret_access_key'.
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// Region is the AWS region to use for route53 operations.
	// This defaults to us-east-1.
	// For AWS China, use cn-northwest-1.
	// +optional
	Region string `json:"region,omitempty"`
}

// ManageDNSGCPConfig contains GCP-specific info to manage a given domain.
type ManageDNSGCPConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// GCP DNS. It will need permission to manage entries in each of the
	// managed domains for this cluster.
	// listed in the parent ManageDNSConfig object.
	// Secret should have a key named 'osServiceAccount.json'.
	// The credentials must specify the project to use.
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`
}

type DeleteProtectionType string

const (
	DeleteProtectionEnabled DeleteProtectionType = "enabled"
)

// ManageDNSAzureConfig contains Azure-specific info to manage a given domain
type ManageDNSAzureConfig struct {
	// CredentialsSecretRef references a secret in the TargetNamespace that will be used to authenticate with
	// Azure DNS. It wil need permission to manage entries in each of the
	// managed domains listed in the parent ManageDNSConfig object.
	// Secret should have a key named 'osServicePrincipal.json'
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// ResourceGroupName specifies the Azure resource group containing the DNS zones
	// for the domains being managed.
	ResourceGroupName string `json:"resourceGroupName"`

	// CloudName is the name of the Azure cloud environment which can be used to configure the Azure SDK
	// with the appropriate Azure API endpoints.
	// If empty, the value is equal to "AzurePublicCloud".
	// +optional
	CloudName azure.CloudEnvironment `json:"cloudName,omitempty"`
}

// ControllerConfig contains the configuration for a controller
type ControllerConfig struct {
	// ConcurrentReconciles specifies number of concurrent reconciles for a controller
	// +optional
	ConcurrentReconciles *int32 `json:"concurrentReconciles,omitempty"`
	// ClientQPS specifies client rate limiter QPS for a controller
	// +optional
	ClientQPS *int32 `json:"clientQPS,omitempty"`
	// ClientBurst specifies client rate limiter burst for a controller
	// +optional
	ClientBurst *int32 `json:"clientBurst,omitempty"`
	// QueueQPS specifies workqueue rate limiter QPS for a controller
	// +optional
	QueueQPS *int32 `json:"queueQPS,omitempty"`
	// QueueBurst specifies workqueue rate limiter burst for a controller
	// +optional
	QueueBurst *int32 `json:"queueBurst,omitempty"`
	// Replicas specifies the number of replicas the specific controller pod should use.
	// This is ONLY for controllers that have been split out into their own pods.
	// This is ignored for all others.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources describes the compute resource requirements of the controller container.
	// This is ONLY for controllers that have been split out into their own pods.
	// This is ignored for all others.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// +kubebuilder:validation:Enum=clusterDeployment;clusterrelocate;clusterstate;clusterversion;controlPlaneCerts;dnsendpoint;dnszone;remoteingress;remotemachineset;machinepool;syncidentityprovider;unreachable;velerobackup;clusterprovision;clusterDeprovision;clusterpool;clusterpoolnamespace;hibernation;clusterclaim;metrics;clustersync
type ControllerName string

func (controllerName ControllerName) String() string {
	return string(controllerName)
}

// ControllerNames is a slice of controller names
type ControllerNames []ControllerName

// Contains says whether or not the controller name is in the slice of controller names.
func (c ControllerNames) Contains(controllerName ControllerName) bool {
	for _, curControllerName := range c {
		if curControllerName == controllerName {
			return true
		}
	}

	return false
}

// WARNING: All the controller names below should also be added to the kubebuilder validation of the type ControllerName
const (
	ClusterClaimControllerName         ControllerName = "clusterclaim"
	ClusterDeploymentControllerName    ControllerName = "clusterDeployment"
	ClusterDeprovisionControllerName   ControllerName = "clusterDeprovision"
	ClusterpoolControllerName          ControllerName = "clusterpool"
	ClusterpoolNamespaceControllerName ControllerName = "clusterpoolnamespace"
	ClusterProvisionControllerName     ControllerName = "clusterProvision"
	ClusterRelocateControllerName      ControllerName = "clusterRelocate"
	ClusterStateControllerName         ControllerName = "clusterState"
	ClusterVersionControllerName       ControllerName = "clusterversion"
	ControlPlaneCertsControllerName    ControllerName = "controlPlaneCerts"
	DNSEndpointControllerName          ControllerName = "dnsendpoint"
	DNSZoneControllerName              ControllerName = "dnszone"
	FakeClusterInstallControllerName   ControllerName = "fakeclusterinstall"
	HibernationControllerName          ControllerName = "hibernation"
	RemoteIngressControllerName        ControllerName = "remoteingress"
	SyncIdentityProviderControllerName ControllerName = "syncidentityprovider"
	UnreachableControllerName          ControllerName = "unreachable"
	VeleroBackupControllerName         ControllerName = "velerobackup"
	MetricsControllerName              ControllerName = "metrics"
	ClustersyncControllerName          ControllerName = "clustersync"
	AWSPrivateLinkControllerName       ControllerName = "awsprivatelink"
	PrivateLinkControllerName          ControllerName = "privatelink"
	HiveControllerName                 ControllerName = "hive"

	// DeprecatedRemoteMachinesetControllerName was deprecated but can be used to disable the
	// MachinePool controller which supercedes it for compatability.
	DeprecatedRemoteMachinesetControllerName ControllerName = "remotemachineset"
	MachinePoolControllerName                ControllerName = "machinepool"
)

// SpecificControllerConfig contains the configuration for a specific controller
type SpecificControllerConfig struct {
	// Name specifies the name of the controller
	Name ControllerName `json:"name"`
	// ControllerConfig contains the configuration for the controller specified by Name field
	Config ControllerConfig `json:"config"`
}

// ControllersConfig contains default as well as controller specific configurations
type ControllersConfig struct {
	// Default specifies default configuration for all the controllers, can be used to override following coded defaults
	// default for concurrent reconciles is 5
	// default for client qps is 5
	// default for client burst is 10
	// default for queue qps is 10
	// default for queue burst is 100
	// +optional
	Default *ControllerConfig `json:"default,omitempty"`
	// Controllers contains a list of configurations for different controllers
	// +optional
	Controllers []SpecificControllerConfig `json:"controllers,omitempty"`
}

type DeploymentName string

const (
	DeploymentNameControllers DeploymentName = "hive-controllers"
	DeploymentNameClustersync DeploymentName = "hive-clustersync"
	DeploymentNameMachinepool DeploymentName = "hive-machinepool"
	DeploymentNameAdmission   DeploymentName = "hiveadmission"
)

type DeploymentConfig struct {
	// DeploymentName is the name of one of the Deployments/StatefulSets managed by hive-operator.
	// NOTE: At this time each deployment has only one container. In the future, we may provide a
	// way to specify which container this DeploymentConfig will be applied to.
	// +kubebuilder:validation:Enum=hive-controllers;hive-clustersync;hiveadmission
	DeploymentName DeploymentName `json:"deploymentName"`

	// Resources allows customization of the resource (memory, CPU, etc.) limits and requests used
	// by containers in the Deployment/StatefulSet named by DeploymentName.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources"`
}

// +genclient:nonNamespaced
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HiveConfig is the Schema for the hives API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type HiveConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HiveConfigSpec   `json:"spec,omitempty"`
	Status HiveConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HiveConfigList contains a list of Hive
type HiveConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HiveConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HiveConfig{}, &HiveConfigList{})
}
