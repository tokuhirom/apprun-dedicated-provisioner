package config

// ClusterConfig represents the YAML configuration for a cluster
type ClusterConfig struct {
	// ClusterName is the target cluster name
	ClusterName string `yaml:"clusterName"`
	// Cluster contains cluster-level settings (optional, for updating existing cluster)
	Cluster *ClusterSettings `yaml:"cluster,omitempty"`
	// AutoScalingGroups is a list of auto scaling group configurations
	AutoScalingGroups []AutoScalingGroupConfig `yaml:"autoScalingGroups,omitempty"`
	// LoadBalancers is a list of load balancer configurations
	LoadBalancers []LoadBalancerConfig `yaml:"loadBalancers,omitempty"`
	// Applications is a list of application configurations
	Applications []ApplicationConfig `yaml:"applications"`
}

// ClusterSettings represents cluster-level settings that can be updated
type ClusterSettings struct {
	// LetsEncryptEmail enables Let's Encrypt certificate issuance
	LetsEncryptEmail *string `yaml:"letsEncryptEmail,omitempty"`
	// ServicePrincipalID is the service principal ID
	ServicePrincipalID string `yaml:"servicePrincipalId"`
}

// AutoScalingGroupConfig represents an auto scaling group configuration
// Note: ASG settings cannot be updated. Changes require delete and recreate.
type AutoScalingGroupConfig struct {
	// Name is the ASG name (must be unique within cluster)
	Name string `yaml:"name"`
	// Zone is the zone where the ASG is created (e.g., "is1a")
	Zone string `yaml:"zone"`
	// WorkerServiceClassPath is the service class path for workers
	WorkerServiceClassPath string `yaml:"workerServiceClassPath"`
	// MinNodes is the minimum number of nodes
	MinNodes int32 `yaml:"minNodes"`
	// MaxNodes is the maximum number of nodes
	MaxNodes int32 `yaml:"maxNodes"`
	// NameServers is the list of DNS servers
	NameServers []string `yaml:"nameServers"`
	// Interfaces is the list of network interfaces
	Interfaces []ASGInterfaceConfig `yaml:"interfaces"`
}

// ASGInterfaceConfig represents a network interface configuration for ASG
type ASGInterfaceConfig struct {
	// InterfaceIndex is the interface number (0=eth0, 1=eth1, etc.)
	InterfaceIndex int16 `yaml:"interfaceIndex"`
	// Upstream is "shared" for shared segment, or switch/router ID
	Upstream string `yaml:"upstream"`
	// IpPool is the IP address pool (required unless upstream is "shared")
	IpPool []IpRangeConfig `yaml:"ipPool,omitempty"`
	// NetmaskLen is the netmask length (required unless upstream is "shared")
	NetmaskLen *int16 `yaml:"netmaskLen,omitempty"`
	// DefaultGateway is the default gateway
	DefaultGateway *string `yaml:"defaultGateway,omitempty"`
	// PacketFilterID is the packet filter ID
	PacketFilterID *string `yaml:"packetFilterId,omitempty"`
	// ConnectsToLB indicates if this interface connects to load balancer
	ConnectsToLB bool `yaml:"connectsToLB"`
}

// IpRangeConfig represents an IP address range
type IpRangeConfig struct {
	// Start is the start IP address
	Start string `yaml:"start"`
	// End is the end IP address
	End string `yaml:"end"`
}

// LoadBalancerConfig represents a load balancer configuration
// Note: LB settings cannot be updated. Changes require delete and recreate.
type LoadBalancerConfig struct {
	// Name is the load balancer name
	Name string `yaml:"name"`
	// AutoScalingGroupName is the name of the ASG this LB belongs to
	AutoScalingGroupName string `yaml:"autoScalingGroupName"`
	// ServiceClassPath is the service class path
	ServiceClassPath string `yaml:"serviceClassPath"`
	// NameServers is the list of DNS servers
	NameServers []string `yaml:"nameServers"`
	// Interfaces is the list of network interfaces
	Interfaces []LBInterfaceConfig `yaml:"interfaces"`
}

// LBInterfaceConfig represents a network interface configuration for LoadBalancer
type LBInterfaceConfig struct {
	// InterfaceIndex is the interface number
	InterfaceIndex int16 `yaml:"interfaceIndex"`
	// Upstream is "shared" for shared segment, or switch/router ID
	Upstream string `yaml:"upstream"`
	// IpPool is the IP address pool (required unless upstream is "shared")
	IpPool []IpRangeConfig `yaml:"ipPool,omitempty"`
	// NetmaskLen is the netmask length (required unless upstream is "shared")
	NetmaskLen *int16 `yaml:"netmaskLen,omitempty"`
	// DefaultGateway is the default gateway
	DefaultGateway *string `yaml:"defaultGateway,omitempty"`
	// Vip is the virtual IP address
	Vip *string `yaml:"vip,omitempty"`
	// VirtualRouterID is the VRRP virtual router ID (1-255, required if vip is set)
	VirtualRouterID *int16 `yaml:"virtualRouterId,omitempty"`
	// PacketFilterID is the packet filter ID
	PacketFilterID *string `yaml:"packetFilterId,omitempty"`
}

// ApplicationConfig represents an application configuration
type ApplicationConfig struct {
	// Name is the application name (must be unique within cluster)
	Name string `yaml:"name"`
	// Spec contains the application spec settings
	Spec ApplicationSpec `yaml:"spec"`
}

// ApplicationSpec represents the application spec settings
type ApplicationSpec struct {
	// CPU in mCPU (100-64000)
	CPU int64 `yaml:"cpu"`
	// Memory in MB (128-131072)
	Memory int64 `yaml:"memory"`
	// ScalingMode: "manual" or "cpu"
	ScalingMode string `yaml:"scalingMode"`
	// FixedScale for manual scaling mode
	FixedScale *int32 `yaml:"fixedScale,omitempty"`
	// MinScale for cpu scaling mode
	MinScale *int32 `yaml:"minScale,omitempty"`
	// MaxScale for cpu scaling mode
	MaxScale *int32 `yaml:"maxScale,omitempty"`
	// ScaleInThreshold for cpu scaling mode (30-70)
	ScaleInThreshold *int32 `yaml:"scaleInThreshold,omitempty"`
	// ScaleOutThreshold for cpu scaling mode (50-99)
	ScaleOutThreshold *int32 `yaml:"scaleOutThreshold,omitempty"`
	// Image is the container image
	Image string `yaml:"image"`
	// Cmd is the command to run (optional)
	Cmd []string `yaml:"cmd,omitempty"`
	// Registry credentials
	RegistryUsername        *string `yaml:"registryUsername,omitempty"`
	RegistryPassword        *string `yaml:"registryPassword,omitempty"`
	RegistryPasswordVersion *int    `yaml:"registryPasswordVersion,omitempty"`
	// ExposedPorts defines ports exposed by the application
	ExposedPorts []ExposedPortConfig `yaml:"exposedPorts"`
	// Env is a list of environment variables
	Env []EnvVarConfig `yaml:"env,omitempty"`
}

// ExposedPortConfig represents a port configuration
type ExposedPortConfig struct {
	// TargetPort is the port the application listens on
	TargetPort int32 `yaml:"targetPort"`
	// LoadBalancerPort is the external port (null if not exposed via LB)
	LoadBalancerPort *int32 `yaml:"loadBalancerPort,omitempty"`
	// UseLetsEncrypt enables Let's Encrypt for HTTPS
	UseLetsEncrypt bool `yaml:"useLetsEncrypt"`
	// Host is the hostname for HTTP/HTTPS routing
	Host []string `yaml:"host,omitempty"`
	// HealthCheck configuration
	HealthCheck *HealthCheckConfig `yaml:"healthCheck,omitempty"`
}

// HealthCheckConfig represents health check settings
type HealthCheckConfig struct {
	// Path is the health check endpoint path
	Path string `yaml:"path"`
	// IntervalSeconds is the check interval in seconds
	IntervalSeconds int32 `yaml:"intervalSeconds"`
	// TimeoutSeconds is the check timeout in seconds
	TimeoutSeconds int32 `yaml:"timeoutSeconds"`
}

// EnvVarConfig represents an environment variable
type EnvVarConfig struct {
	// Key is the environment variable name
	Key string `yaml:"key"`
	// Value is the environment variable value
	Value *string `yaml:"value,omitempty"`
	// Secret marks the variable as secret (value cannot be retrieved via API)
	Secret bool `yaml:"secret"`
	// SecretVersion is required when secret is true (increment to trigger update)
	SecretVersion *int `yaml:"secretVersion,omitempty"`
}
