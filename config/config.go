package config

// ClusterConfig represents the YAML configuration for a cluster
type ClusterConfig struct {
	// ClusterName is the target cluster name
	ClusterName string `yaml:"clusterName"`
	// Applications is a list of application configurations
	Applications []ApplicationConfig `yaml:"applications"`
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
	RegistryUsername *string `yaml:"registryUsername,omitempty"`
	RegistryPassword *string `yaml:"registryPassword,omitempty"`
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
}
