package provisioner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/r3labs/diff/v3"

	"github.com/tokuhirom/apprun-dedicated-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-provisioner/config"
)

// NormalizedSpec is a normalized representation of application spec for comparison.
// Both API responses and config structs are converted to this format before comparison.
// Note: Env and RegistryPassword are handled separately due to state file integration.
type NormalizedSpec struct {
	CPU               int64  `diff:"CPU"`
	Memory            int64  `diff:"Memory"`
	ScalingMode       string `diff:"ScalingMode"`
	FixedScale        *int32 `diff:"FixedScale"`
	MinScale          *int32 `diff:"MinScale"`
	MaxScale          *int32 `diff:"MaxScale"`
	ScaleInThreshold  *int32 `diff:"ScaleInThreshold"`
	ScaleOutThreshold *int32 `diff:"ScaleOutThreshold"`
	Image             string `diff:"Image"`
	Cmd               string `diff:"Cmd"` // Joined with space for easier comparison
	RegistryUsername  string `diff:"RegistryUsername"`

	ExposedPorts []NormalizedExposedPort `diff:"ExposedPorts"`
}

// NormalizedExposedPort is a normalized representation of exposed port
type NormalizedExposedPort struct {
	TargetPort          int32  `diff:"TargetPort,identifier"`
	LoadBalancerPort    *int32 `diff:"LoadBalancerPort"`
	UseLetsEncrypt      bool   `diff:"UseLetsEncrypt"`
	Hosts               string `diff:"Hosts"` // Joined and sorted for consistent comparison
	HealthCheckPath     string `diff:"HealthCheckPath"`
	HealthCheckInterval int32  `diff:"HealthCheckInterval"`
	HealthCheckTimeout  int32  `diff:"HealthCheckTimeout"`
}

// NormalizeFromAPI converts API response to normalized spec
func NormalizeFromAPI(v *api.ReadApplicationVersionDetail) *NormalizedSpec {
	spec := &NormalizedSpec{
		CPU:         v.CPU,
		Memory:      v.Memory,
		ScalingMode: string(v.ScalingMode),
		Image:       v.Image,
		Cmd:         strings.Join(v.Cmd, " "),
	}

	// Handle optional fields
	if val, ok := v.FixedScale.Get(); ok {
		spec.FixedScale = &val
	}
	if val, ok := v.MinScale.Get(); ok {
		spec.MinScale = &val
	}
	if val, ok := v.MaxScale.Get(); ok {
		spec.MaxScale = &val
	}
	if val, ok := v.ScaleInThreshold.Get(); ok {
		spec.ScaleInThreshold = &val
	}
	if val, ok := v.ScaleOutThreshold.Get(); ok {
		spec.ScaleOutThreshold = &val
	}
	if !v.RegistryUsername.IsNull() {
		spec.RegistryUsername = v.RegistryUsername.Value
	}

	// Normalize exposed ports
	for _, p := range v.ExposedPorts {
		np := NormalizedExposedPort{
			TargetPort:     int32(p.TargetPort),
			UseLetsEncrypt: p.UseLetsEncrypt,
		}
		if !p.LoadBalancerPort.IsNull() {
			val := int32(p.LoadBalancerPort.Value)
			np.LoadBalancerPort = &val
		}
		// Sort hosts for consistent comparison
		hosts := make([]string, len(p.Host))
		copy(hosts, p.Host)
		sort.Strings(hosts)
		np.Hosts = strings.Join(hosts, ",")

		if hc, ok := p.HealthCheck.Get(); ok {
			np.HealthCheckPath = hc.Path
			np.HealthCheckInterval = hc.IntervalSeconds
			np.HealthCheckTimeout = hc.TimeoutSeconds
		}
		spec.ExposedPorts = append(spec.ExposedPorts, np)
	}

	// Sort for consistent comparison
	sortNormalizedSpec(spec)

	return spec
}

// NormalizeFromConfig converts config struct to normalized spec
func NormalizeFromConfig(c *config.ApplicationSpec) *NormalizedSpec {
	spec := &NormalizedSpec{
		CPU:               c.CPU,
		Memory:            c.Memory,
		ScalingMode:       c.ScalingMode,
		FixedScale:        c.FixedScale,
		MinScale:          c.MinScale,
		MaxScale:          c.MaxScale,
		ScaleInThreshold:  c.ScaleInThreshold,
		ScaleOutThreshold: c.ScaleOutThreshold,
		Image:             c.Image,
		Cmd:               strings.Join(c.Cmd, " "),
	}

	if c.RegistryUsername != nil {
		spec.RegistryUsername = *c.RegistryUsername
	}

	// Normalize exposed ports
	for _, p := range c.ExposedPorts {
		np := NormalizedExposedPort{
			TargetPort:       p.TargetPort,
			LoadBalancerPort: p.LoadBalancerPort,
			UseLetsEncrypt:   p.UseLetsEncrypt,
		}
		// Sort hosts for consistent comparison
		hosts := make([]string, len(p.Host))
		copy(hosts, p.Host)
		sort.Strings(hosts)
		np.Hosts = strings.Join(hosts, ",")

		if p.HealthCheck != nil {
			np.HealthCheckPath = p.HealthCheck.Path
			np.HealthCheckInterval = p.HealthCheck.IntervalSeconds
			np.HealthCheckTimeout = p.HealthCheck.TimeoutSeconds
		}
		spec.ExposedPorts = append(spec.ExposedPorts, np)
	}

	// Sort for consistent comparison
	sortNormalizedSpec(spec)

	return spec
}

// sortNormalizedSpec sorts slices for consistent comparison
func sortNormalizedSpec(spec *NormalizedSpec) {
	sort.Slice(spec.ExposedPorts, func(i, j int) bool {
		return spec.ExposedPorts[i].TargetPort < spec.ExposedPorts[j].TargetPort
	})
}

// CompareSpecsOptions configures the comparison behavior
type CompareSpecsOptions struct {
	// SkipImage skips Image field comparison (used in plan command where image is inherited)
	SkipImage bool
}

// CompareSpecs compares two normalized specs and returns human-readable changes
func CompareSpecs(from, to *NormalizedSpec, opts CompareSpecsOptions) ([]string, error) {
	changelog, err := diff.Diff(from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to diff specs: %w", err)
	}

	changes := make([]string, 0, len(changelog))
	for _, change := range changelog {
		// Skip Image field if requested
		if opts.SkipImage && len(change.Path) > 0 && change.Path[0] == "Image" {
			continue
		}
		changes = append(changes, formatChange(change))
	}

	return changes, nil
}

// formatChange converts a diff.Change to a human-readable string
func formatChange(c diff.Change) string {
	path := strings.Join(c.Path, ".")

	switch c.Type {
	case diff.CREATE:
		return fmt.Sprintf("%s: (unset) -> %v", path, formatValue(c.To))
	case diff.UPDATE:
		return fmt.Sprintf("%s: %v -> %v", path, formatValue(c.From), formatValue(c.To))
	case diff.DELETE:
		return fmt.Sprintf("%s: %v -> (unset)", path, formatValue(c.From))
	default:
		return fmt.Sprintf("%s: %v -> %v", path, formatValue(c.From), formatValue(c.To))
	}
}

// formatValue formats a value for display, dereferencing pointers
func formatValue(v interface{}) interface{} {
	if v == nil {
		return "(nil)"
	}
	switch val := v.(type) {
	case *int32:
		if val == nil {
			return "(nil)"
		}
		return *val
	case *int64:
		if val == nil {
			return "(nil)"
		}
		return *val
	case *string:
		if val == nil {
			return "(nil)"
		}
		return *val
	case *bool:
		if val == nil {
			return "(nil)"
		}
		return *val
	default:
		return v
	}
}
