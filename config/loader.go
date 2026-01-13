package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file
func Load(path string) (*ClusterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// validate checks if the configuration is valid
func validate(config *ClusterConfig) error {
	if config.ClusterName == "" {
		return fmt.Errorf("clusterName is required")
	}
	if len(config.Applications) == 0 {
		return fmt.Errorf("at least one application is required")
	}

	for i, app := range config.Applications {
		if err := validateApplication(&app, i); err != nil {
			return err
		}
	}

	return nil
}

func validateApplication(app *ApplicationConfig, index int) error {
	if app.Name == "" {
		return fmt.Errorf("applications[%d]: name is required", index)
	}

	v := &app.Spec
	if v.CPU < 100 || v.CPU > 64000 {
		return fmt.Errorf("applications[%d]: cpu must be between 100 and 64000", index)
	}
	if v.Memory < 128 || v.Memory > 131072 {
		return fmt.Errorf("applications[%d]: memory must be between 128 and 131072", index)
	}
	if v.ScalingMode != "manual" && v.ScalingMode != "cpu" {
		return fmt.Errorf("applications[%d]: scalingMode must be 'manual' or 'cpu'", index)
	}
	if v.Image == "" {
		return fmt.Errorf("applications[%d]: image is required", index)
	}
	if len(v.ExposedPorts) == 0 {
		return fmt.Errorf("applications[%d]: at least one exposed port is required", index)
	}

	// Validate scaling parameters
	if v.ScalingMode == "manual" {
		if v.FixedScale == nil {
			return fmt.Errorf("applications[%d]: fixedScale is required when scalingMode is 'manual'", index)
		}
	} else if v.ScalingMode == "cpu" {
		if v.MinScale == nil || v.MaxScale == nil {
			return fmt.Errorf("applications[%d]: minScale and maxScale are required when scalingMode is 'cpu'", index)
		}
	}

	// Validate registry credentials
	if v.RegistryPassword != nil && v.RegistryPasswordVersion == nil {
		return fmt.Errorf("applications[%d]: registryPasswordVersion is required when registryPassword is specified", index)
	}

	return nil
}
