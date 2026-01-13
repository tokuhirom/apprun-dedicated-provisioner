package provisioner

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
)

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionUpdate ActionType = "update"
	ActionNoop   ActionType = "noop"
)

// PlannedAction represents a planned change
type PlannedAction struct {
	ApplicationName string
	Action          ActionType
	Changes         []string // Description of changes
}

// Plan represents the execution plan
type Plan struct {
	ClusterName string
	ClusterID   uuid.UUID
	Actions     []PlannedAction
}

// ApplyOptions contains options for the Apply operation
type ApplyOptions struct {
	// Activate determines whether to activate the version after creating/updating.
	// If false (default), only creates/updates the version without activating.
	// If true, also activates the version.
	Activate bool
}

// Provisioner handles the synchronization of application configurations
type Provisioner struct {
	client *api.Client
}

// NewProvisioner creates a new Provisioner
func NewProvisioner(client *api.Client) *Provisioner {
	return &Provisioner{
		client: client,
	}
}

// CreatePlan creates an execution plan by comparing config with current state
func (p *Provisioner) CreatePlan(ctx context.Context, cfg *config.ClusterConfig) (*Plan, error) {
	// Resolve cluster name to ID
	clusterID, err := p.resolveClusterID(ctx, cfg.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cluster: %w", err)
	}

	plan := &Plan{
		ClusterName: cfg.ClusterName,
		ClusterID:   clusterID,
	}

	// Get existing applications
	existing, err := p.listAllApplications(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	// Build a map of existing applications by name
	existingByName := make(map[string]*api.ReadApplicationDetail)
	for i := range existing {
		existingByName[existing[i].Name] = existing[i]
	}

	// Process each application in the config
	for _, appCfg := range cfg.Applications {
		if existingApp, ok := existingByName[appCfg.Name]; ok {
			// Application exists, check if update is needed
			action, err := p.planUpdate(ctx, existingApp, &appCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to plan update for %s: %w", appCfg.Name, err)
			}
			plan.Actions = append(plan.Actions, *action)
			delete(existingByName, appCfg.Name)
		} else {
			// Application doesn't exist, plan to create it
			plan.Actions = append(plan.Actions, PlannedAction{
				ApplicationName: appCfg.Name,
				Action:          ActionCreate,
				Changes:         []string{"Create new application and version"},
			})
		}
	}

	// Warn about applications not in config
	for name := range existingByName {
		log.Printf("WARNING: Application %q exists in AppRun but not in config", name)
	}

	return plan, nil
}

// Apply executes the given plan
func (p *Provisioner) Apply(ctx context.Context, cfg *config.ClusterConfig, plan *Plan, opts ApplyOptions) error {
	// Use cluster ID from the plan (already resolved)
	clusterID := plan.ClusterID

	// Get existing applications for lookup
	existing, err := p.listAllApplications(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	existingByName := make(map[string]*api.ReadApplicationDetail)
	for i := range existing {
		existingByName[existing[i].Name] = existing[i]
	}

	// Build config map
	configByName := make(map[string]*config.ApplicationConfig)
	for i := range cfg.Applications {
		configByName[cfg.Applications[i].Name] = &cfg.Applications[i]
	}

	for _, action := range plan.Actions {
		appCfg, ok := configByName[action.ApplicationName]
		if !ok {
			continue
		}

		switch action.Action {
		case ActionCreate:
			if err := p.createApplication(ctx, clusterID, appCfg, opts); err != nil {
				return fmt.Errorf("failed to create application %s: %w", action.ApplicationName, err)
			}
		case ActionUpdate:
			existingApp := existingByName[action.ApplicationName]
			if err := p.updateApplication(ctx, existingApp, appCfg, opts); err != nil {
				return fmt.Errorf("failed to update application %s: %w", action.ApplicationName, err)
			}
		case ActionNoop:
			log.Printf("Application %q is up to date", action.ApplicationName)
		}
	}

	return nil
}

// planUpdate checks what changes would be needed for an existing application
func (p *Provisioner) planUpdate(ctx context.Context, existing *api.ReadApplicationDetail, appCfg *config.ApplicationConfig) (*PlannedAction, error) {
	action := &PlannedAction{
		ApplicationName: appCfg.Name,
		Action:          ActionNoop,
	}

	// Get the latest version
	latestVersion, err := p.getLatestVersion(ctx, existing.ApplicationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	if latestVersion == nil {
		action.Action = ActionUpdate
		action.Changes = append(action.Changes, "Create initial version (no versions exist)")
		return action, nil
	}

	// Compare settings (excluding image)
	changes := p.compareVersion(latestVersion, &appCfg.Spec)
	if len(changes) > 0 {
		action.Action = ActionUpdate
		action.Changes = changes
	}

	return action, nil
}

// compareVersion compares the current version with desired config and returns list of changes
func (p *Provisioner) compareVersion(current *api.ReadApplicationVersionDetail, desired *config.ApplicationSpec) []string {
	var changes []string

	if current.CPU != desired.CPU {
		changes = append(changes, fmt.Sprintf("CPU: %d -> %d", current.CPU, desired.CPU))
	}
	if current.Memory != desired.Memory {
		changes = append(changes, fmt.Sprintf("Memory: %d -> %d", current.Memory, desired.Memory))
	}
	if string(current.ScalingMode) != desired.ScalingMode {
		changes = append(changes, fmt.Sprintf("ScalingMode: %s -> %s", current.ScalingMode, desired.ScalingMode))
	}

	// Compare scaling parameters
	if desired.ScalingMode == "manual" && desired.FixedScale != nil {
		if v, ok := current.FixedScale.Get(); !ok {
			changes = append(changes, fmt.Sprintf("FixedScale: (unset) -> %d", *desired.FixedScale))
		} else if v != *desired.FixedScale {
			changes = append(changes, fmt.Sprintf("FixedScale: %d -> %d", v, *desired.FixedScale))
		}
	}
	if desired.ScalingMode == "cpu" {
		if desired.MinScale != nil {
			if v, ok := current.MinScale.Get(); !ok {
				changes = append(changes, fmt.Sprintf("MinScale: (unset) -> %d", *desired.MinScale))
			} else if v != *desired.MinScale {
				changes = append(changes, fmt.Sprintf("MinScale: %d -> %d", v, *desired.MinScale))
			}
		}
		if desired.MaxScale != nil {
			if v, ok := current.MaxScale.Get(); !ok {
				changes = append(changes, fmt.Sprintf("MaxScale: (unset) -> %d", *desired.MaxScale))
			} else if v != *desired.MaxScale {
				changes = append(changes, fmt.Sprintf("MaxScale: %d -> %d", v, *desired.MaxScale))
			}
		}
	}

	// Compare exposed ports count
	if len(current.ExposedPorts) != len(desired.ExposedPorts) {
		changes = append(changes, fmt.Sprintf("ExposedPorts count: %d -> %d", len(current.ExposedPorts), len(desired.ExposedPorts)))
	}

	// Compare env count
	if len(current.Env) != len(desired.Env) {
		changes = append(changes, fmt.Sprintf("Env count: %d -> %d", len(current.Env), len(desired.Env)))
	}

	return changes
}

// resolveClusterID resolves a cluster name to its ID
func (p *Provisioner) resolveClusterID(ctx context.Context, clusterName string) (uuid.UUID, error) {
	var cursor api.OptClusterID

	for {
		resp, err := p.client.ListClusters(ctx, api.ListClustersParams{
			MaxItems: 30,
			Cursor:   cursor,
		})
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("failed to list clusters: %w", err)
		}

		for _, cluster := range resp.Clusters {
			if cluster.Name == clusterName {
				return uuid.UUID(cluster.ClusterID), nil
			}
		}

		if resp.NextCursor.Set {
			cursor = resp.NextCursor
		} else {
			break
		}
	}

	return uuid.UUID{}, fmt.Errorf("cluster %q not found", clusterName)
}

// listAllApplications fetches all applications for the given cluster
func (p *Provisioner) listAllApplications(ctx context.Context, clusterID uuid.UUID) ([]*api.ReadApplicationDetail, error) {
	var apps []*api.ReadApplicationDetail
	var cursor api.OptString

	for {
		resp, err := p.client.ListApplications(ctx, api.ListApplicationsParams{
			ClusterID: api.OptClusterID{Value: api.ClusterID(clusterID), Set: true},
			MaxItems:  30,
			Cursor:    cursor,
		})
		if err != nil {
			return nil, err
		}

		for i := range resp.Applications {
			apps = append(apps, &resp.Applications[i])
		}

		if resp.NextCursor.Set && resp.NextCursor.Value != "" {
			cursor = resp.NextCursor
		} else {
			break
		}
	}

	return apps, nil
}

// getLatestVersion returns the latest version of the application
func (p *Provisioner) getLatestVersion(ctx context.Context, appID api.ApplicationID) (*api.ReadApplicationVersionDetail, error) {
	resp, err := p.client.ListApplicationVersions(ctx, api.ListApplicationVersionsParams{
		ApplicationID: appID,
		MaxItems:      30,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Versions) == 0 {
		return nil, nil
	}

	// Find the latest version (highest version number)
	var latestVersionNum api.ApplicationVersionNumber
	for _, v := range resp.Versions {
		if v.Version > latestVersionNum {
			latestVersionNum = v.Version
		}
	}

	// Get the full details of the latest version
	versionResp, err := p.client.GetApplicationVersion(ctx, api.GetApplicationVersionParams{
		ApplicationID: appID,
		Version:       latestVersionNum,
	})
	if err != nil {
		return nil, err
	}

	return &versionResp.ApplicationVersion, nil
}

// createApplication creates a new application with the given configuration
func (p *Provisioner) createApplication(ctx context.Context, clusterID uuid.UUID, appCfg *config.ApplicationConfig, opts ApplyOptions) error {
	log.Printf("Creating application %q", appCfg.Name)

	// Create the application
	createResp, err := p.client.CreateApplication(ctx, &api.CreateApplication{
		Name:      appCfg.Name,
		ClusterID: api.ClusterID(clusterID),
	})
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	appID := createResp.Application.ApplicationID
	log.Printf("Created application %q with ID %s", appCfg.Name, uuid.UUID(appID))

	// Create the version (using image from config for new applications)
	versionReq := p.buildCreateVersionRequest(&appCfg.Spec)
	versionResp, err := p.client.CreateApplicationVersion(ctx, versionReq, api.CreateApplicationVersionParams{
		ApplicationID: appID,
	})
	if err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	versionNum := versionResp.ApplicationVersion.Version
	log.Printf("Created version %d for application %q", versionNum, appCfg.Name)

	// Activate the version only if requested
	if opts.Activate {
		updateReq := &api.UpdateApplication{}
		updateReq.ActiveVersion.SetTo(int32(versionNum))
		err = p.client.UpdateApplication(ctx, updateReq, api.UpdateApplicationParams{
			ApplicationID: appID,
		})
		if err != nil {
			return fmt.Errorf("failed to activate version: %w", err)
		}

		log.Printf("Activated version %d for application %q", versionNum, appCfg.Name)
	} else {
		log.Printf("Skipped activation for application %q (use --activate to activate)", appCfg.Name)
	}
	return nil
}

// updateApplication creates a new version and optionally activates it
func (p *Provisioner) updateApplication(ctx context.Context, existing *api.ReadApplicationDetail, appCfg *config.ApplicationConfig, opts ApplyOptions) error {
	log.Printf("Updating application %q", appCfg.Name)

	// Get the latest version to inherit settings
	latestVersion, err := p.getLatestVersion(ctx, existing.ApplicationID)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	// Create the new version (merge with existing settings)
	versionReq := p.buildCreateVersionRequestWithBase(&appCfg.Spec, latestVersion)
	versionResp, err := p.client.CreateApplicationVersion(ctx, versionReq, api.CreateApplicationVersionParams{
		ApplicationID: existing.ApplicationID,
	})
	if err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	versionNum := versionResp.ApplicationVersion.Version
	log.Printf("Created version %d for application %q", versionNum, appCfg.Name)

	// Activate the version only if requested
	if opts.Activate {
		updateReq := &api.UpdateApplication{}
		updateReq.ActiveVersion.SetTo(int32(versionNum))
		err = p.client.UpdateApplication(ctx, updateReq, api.UpdateApplicationParams{
			ApplicationID: existing.ApplicationID,
		})
		if err != nil {
			return fmt.Errorf("failed to activate version: %w", err)
		}

		log.Printf("Activated version %d for application %q", versionNum, appCfg.Name)
	} else {
		log.Printf("Skipped activation for application %q (use --activate to activate)", appCfg.Name)
	}
	return nil
}

// buildCreateVersionRequest builds the API request for creating a version (for new applications)
func (p *Provisioner) buildCreateVersionRequest(v *config.ApplicationSpec) *api.CreateApplicationVersion {
	return p.buildCreateVersionRequestWithBase(v, nil)
}

// buildCreateVersionRequestWithBase builds the API request, merging with existing version settings
func (p *Provisioner) buildCreateVersionRequestWithBase(v *config.ApplicationSpec, base *api.ReadApplicationVersionDetail) *api.CreateApplicationVersion {
	req := &api.CreateApplicationVersion{}

	// Image: always use existing if available, otherwise from config
	if base != nil {
		req.Image = base.Image
	} else {
		req.Image = v.Image
	}

	// CPU: use config if specified (non-zero), otherwise inherit
	if v.CPU != 0 {
		req.CPU = v.CPU
	} else if base != nil {
		req.CPU = base.CPU
	}

	// Memory: use config if specified (non-zero), otherwise inherit
	if v.Memory != 0 {
		req.Memory = v.Memory
	} else if base != nil {
		req.Memory = base.Memory
	}

	// ScalingMode: use config if specified (non-empty), otherwise inherit
	if v.ScalingMode != "" {
		req.ScalingMode = api.ScalingMode(v.ScalingMode)
	} else if base != nil {
		req.ScalingMode = base.ScalingMode
	}

	// Cmd: use config if specified, otherwise inherit
	if len(v.Cmd) > 0 {
		req.Cmd = v.Cmd
	} else if base != nil {
		req.Cmd = base.Cmd
	}

	// Registry credentials
	if v.RegistryUsername != nil {
		req.RegistryUsername.SetTo(*v.RegistryUsername)
		req.RegistryPasswordAction = api.RegistryPasswordActionNew
	} else if base != nil && base.RegistryUsername.Value != "" {
		req.RegistryUsername.SetTo(base.RegistryUsername.Value)
		req.RegistryPasswordAction = api.RegistryPasswordActionKeep
	} else {
		req.RegistryUsername.SetToNull()
		req.RegistryPasswordAction = api.RegistryPasswordActionRemove
	}

	if v.RegistryPassword != nil {
		req.RegistryPassword.SetTo(*v.RegistryPassword)
		req.RegistryPasswordAction = api.RegistryPasswordActionNew
	} else if base != nil && !base.RegistryUsername.IsNull() {
		// Keep existing password if we have existing credentials
		req.RegistryPasswordAction = api.RegistryPasswordActionKeep
	} else {
		req.RegistryPassword.SetToNull()
	}

	// Scaling parameters: use config if specified, otherwise inherit
	if v.FixedScale != nil {
		req.FixedScale = api.OptInt32{Value: *v.FixedScale, Set: true}
	} else if base != nil && base.FixedScale.Set {
		req.FixedScale = base.FixedScale
	}

	if v.MinScale != nil {
		req.MinScale = api.OptInt32{Value: *v.MinScale, Set: true}
	} else if base != nil && base.MinScale.Set {
		req.MinScale = base.MinScale
	}

	if v.MaxScale != nil {
		req.MaxScale = api.OptInt32{Value: *v.MaxScale, Set: true}
	} else if base != nil && base.MaxScale.Set {
		req.MaxScale = base.MaxScale
	}

	if v.ScaleInThreshold != nil {
		req.ScaleInThreshold = api.OptInt32{Value: *v.ScaleInThreshold, Set: true}
	} else if base != nil && base.ScaleInThreshold.Set {
		req.ScaleInThreshold = base.ScaleInThreshold
	}

	if v.ScaleOutThreshold != nil {
		req.ScaleOutThreshold = api.OptInt32{Value: *v.ScaleOutThreshold, Set: true}
	} else if base != nil && base.ScaleOutThreshold.Set {
		req.ScaleOutThreshold = base.ScaleOutThreshold
	}

	// ExposedPorts: use config if specified, otherwise inherit
	if len(v.ExposedPorts) > 0 {
		for _, port := range v.ExposedPorts {
			ep := api.ExposedPort{
				TargetPort:     api.Port(port.TargetPort),
				UseLetsEncrypt: port.UseLetsEncrypt,
				Host:           port.Host,
			}
			if port.LoadBalancerPort != nil {
				ep.LoadBalancerPort.SetTo(api.Port(*port.LoadBalancerPort))
			} else {
				ep.LoadBalancerPort.SetToNull()
			}
			if port.HealthCheck != nil {
				ep.HealthCheck.SetTo(api.HealthCheck{
					Path:            port.HealthCheck.Path,
					IntervalSeconds: port.HealthCheck.IntervalSeconds,
					TimeoutSeconds:  port.HealthCheck.TimeoutSeconds,
				})
			} else {
				ep.HealthCheck.SetToNull()
			}
			req.ExposedPorts = append(req.ExposedPorts, ep)
		}
	} else if base != nil {
		// Inherit from existing version
		for _, port := range base.ExposedPorts {
			ep := api.ExposedPort{
				TargetPort:       port.TargetPort,
				UseLetsEncrypt:   port.UseLetsEncrypt,
				Host:             port.Host,
				LoadBalancerPort: port.LoadBalancerPort,
				HealthCheck:      port.HealthCheck,
			}
			req.ExposedPorts = append(req.ExposedPorts, ep)
		}
	}

	// Env: use config if specified, otherwise inherit
	if len(v.Env) > 0 {
		for _, env := range v.Env {
			e := api.CreateEnvironmentVariable{
				Key:    env.Key,
				Secret: env.Secret,
			}
			if env.Value != nil {
				e.Value = api.OptString{Value: *env.Value, Set: true}
			}
			req.Env = append(req.Env, e)
		}
	} else if base != nil {
		// Inherit from existing version
		for _, env := range base.Env {
			e := api.CreateEnvironmentVariable{
				Key:    env.Key,
				Secret: env.Secret,
			}
			// For secret values, we don't have the value, so don't set it
			// The API should handle this with RegistryPasswordAction-like mechanism
			if !env.Secret && !env.Value.IsNull() {
				e.Value = api.OptString{Value: env.Value.Value, Set: true}
			}
			req.Env = append(req.Env, e)
		}
	}

	return req
}
