package provisioner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/state"
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

// VersionInfo contains information about a single version
type VersionInfo struct {
	Version     int
	Image       string
	Created     time.Time
	ActiveNodes int64
	IsActive    bool
}

// VersionList contains the list of versions for an application
type VersionList struct {
	ApplicationName string
	ApplicationID   string
	Versions        []VersionInfo
	ActiveVersion   int // 0 if no active version
	LatestVersion   int // 0 if no versions exist
}

// VersionDiff contains the differences between two versions
type VersionDiff struct {
	FromVersion    int
	ToVersion      int
	Changes        []string
	HasSecretEnv   bool // true if secret env vars exist (values cannot be compared)
	HasRegistryPwd bool // true if registryPassword exists (value cannot be compared)
}

// Provisioner handles the synchronization of application configurations
type Provisioner struct {
	client     *api.Client
	state      *state.State
	configPath string
}

// NewProvisioner creates a new Provisioner
func NewProvisioner(client *api.Client, st *state.State, configPath string) *Provisioner {
	return &Provisioner{
		client:     client,
		state:      st,
		configPath: configPath,
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
		return nil, wrapAPIError(err, "failed to list applications")
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
		return wrapAPIError(err, "failed to list applications")
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

	stateModified := false

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
			// Update state with password version
			if appCfg.Spec.RegistryPasswordVersion != nil {
				p.state.SetPasswordVersion(appCfg.Name, appCfg.Spec.RegistryPasswordVersion)
				stateModified = true
			}
			// Update state with secret env versions
			if p.updateSecretEnvVersions(appCfg) {
				stateModified = true
			}
		case ActionUpdate:
			existingApp := existingByName[action.ApplicationName]
			if err := p.updateApplication(ctx, existingApp, appCfg, opts); err != nil {
				return fmt.Errorf("failed to update application %s: %w", action.ApplicationName, err)
			}
			// Update state with password version
			storedVersion := p.state.GetPasswordVersion(appCfg.Name)
			desiredVersion := appCfg.Spec.RegistryPasswordVersion
			if desiredVersion != nil {
				if storedVersion == nil || *storedVersion != *desiredVersion {
					p.state.SetPasswordVersion(appCfg.Name, desiredVersion)
					stateModified = true
				}
			} else if storedVersion != nil {
				// Remove version if password was removed
				p.state.SetPasswordVersion(appCfg.Name, nil)
				stateModified = true
			}
			// Update state with secret env versions
			if p.updateSecretEnvVersions(appCfg) {
				stateModified = true
			}
		case ActionNoop:
			log.Printf("Application %q is up to date", action.ApplicationName)
		}
	}

	// Save state file if modified
	if stateModified {
		if err := p.state.Save(p.configPath); err != nil {
			return fmt.Errorf("failed to save state file: %w", err)
		}
		log.Printf("State file updated: %s", state.GetStatePath(p.configPath))
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
		return nil, wrapAPIError(err, "failed to get latest version")
	}

	if latestVersion == nil {
		action.Action = ActionUpdate
		action.Changes = append(action.Changes, "Create initial version (no versions exist)")
		return action, nil
	}

	// Compare settings (excluding image)
	changes := p.compareVersion(appCfg.Name, latestVersion, &appCfg.Spec)
	if len(changes) > 0 {
		action.Action = ActionUpdate
		action.Changes = changes
	}

	return action, nil
}

// compareVersion compares the current version with desired config and returns list of changes
func (p *Provisioner) compareVersion(appName string, current *api.ReadApplicationVersionDetail, desired *config.ApplicationSpec) []string {
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

	// Compare exposed ports
	portChanges := p.compareExposedPorts(current.ExposedPorts, desired.ExposedPorts)
	changes = append(changes, portChanges...)

	// Compare env variables
	envChanges := p.compareEnv(appName, current.Env, desired.Env)
	changes = append(changes, envChanges...)

	// Compare Cmd
	if !stringSlicesEqual(current.Cmd, desired.Cmd) {
		changes = append(changes, fmt.Sprintf("Cmd: %v -> %v", current.Cmd, desired.Cmd))
	}

	// Compare registry credentials
	serverHasRegistryUser := !current.RegistryUsername.IsNull() && current.RegistryUsername.Value != ""
	desiredHasRegistryUser := desired.RegistryUsername != nil && *desired.RegistryUsername != ""

	if !serverHasRegistryUser && desiredHasRegistryUser {
		changes = append(changes, fmt.Sprintf("RegistryUsername: (unset) -> %s", *desired.RegistryUsername))
	} else if serverHasRegistryUser && !desiredHasRegistryUser {
		changes = append(changes, fmt.Sprintf("RegistryUsername: %s -> (unset)", current.RegistryUsername.Value))
	} else if serverHasRegistryUser && desiredHasRegistryUser && current.RegistryUsername.Value != *desired.RegistryUsername {
		changes = append(changes, fmt.Sprintf("RegistryUsername: %s -> %s", current.RegistryUsername.Value, *desired.RegistryUsername))
	}

	// Compare registry password version using state file
	storedVersion := p.state.GetPasswordVersion(appName)
	desiredVersion := desired.RegistryPasswordVersion

	if desiredVersion != nil {
		if storedVersion == nil {
			changes = append(changes, fmt.Sprintf("RegistryPasswordVersion: (new) -> %d", *desiredVersion))
		} else if *storedVersion != *desiredVersion {
			changes = append(changes, fmt.Sprintf("RegistryPasswordVersion: %d -> %d", *storedVersion, *desiredVersion))
		}
		// If versions match, no change needed
	} else if storedVersion != nil {
		// Password was removed from YAML
		changes = append(changes, fmt.Sprintf("RegistryPasswordVersion: %d -> (removed)", *storedVersion))
	}

	return changes
}

// compareEnv compares environment variables and returns list of changes
func (p *Provisioner) compareEnv(appName string, current []api.ReadEnvironmentVariable, desired []config.EnvVarConfig) []string {
	var changes []string

	// Build maps for comparison
	currentByKey := make(map[string]api.ReadEnvironmentVariable)
	for _, env := range current {
		currentByKey[env.Key] = env
	}

	desiredByKey := make(map[string]config.EnvVarConfig)
	for _, env := range desired {
		desiredByKey[env.Key] = env
	}

	// Check for added and changed env vars
	for _, desiredEnv := range desired {
		currentEnv, exists := currentByKey[desiredEnv.Key]
		if !exists {
			// New env var
			if desiredEnv.Secret {
				changes = append(changes, fmt.Sprintf("Env add: %s (secret)", desiredEnv.Key))
			} else if desiredEnv.Value != nil {
				changes = append(changes, fmt.Sprintf("Env add: %s=%s", desiredEnv.Key, *desiredEnv.Value))
			} else {
				changes = append(changes, fmt.Sprintf("Env add: %s", desiredEnv.Key))
			}
			continue
		}

		// Env var exists, check for changes
		if desiredEnv.Secret {
			// For secrets, compare using secretVersion in state file
			storedVersion := p.state.GetSecretEnvVersion(appName, desiredEnv.Key)
			if desiredEnv.SecretVersion != nil {
				if storedVersion == nil {
					changes = append(changes, fmt.Sprintf("Env update: %s (secret, version: new -> %d)", desiredEnv.Key, *desiredEnv.SecretVersion))
				} else if *storedVersion != *desiredEnv.SecretVersion {
					changes = append(changes, fmt.Sprintf("Env update: %s (secret, version: %d -> %d)", desiredEnv.Key, *storedVersion, *desiredEnv.SecretVersion))
				}
			}
		} else {
			// For non-secrets, compare values
			currentValue := ""
			if !currentEnv.Value.IsNull() {
				currentValue = currentEnv.Value.Value
			}
			desiredValue := ""
			if desiredEnv.Value != nil {
				desiredValue = *desiredEnv.Value
			}
			if currentValue != desiredValue {
				changes = append(changes, fmt.Sprintf("Env update: %s=%s -> %s", desiredEnv.Key, currentValue, desiredValue))
			}
		}
	}

	// Check for removed env vars
	for _, currentEnv := range current {
		if _, exists := desiredByKey[currentEnv.Key]; !exists {
			if currentEnv.Secret {
				changes = append(changes, fmt.Sprintf("Env remove: %s (secret)", currentEnv.Key))
			} else {
				changes = append(changes, fmt.Sprintf("Env remove: %s", currentEnv.Key))
			}
		}
	}

	return changes
}

// compareExposedPorts compares exposed port configurations and returns list of changes
func (p *Provisioner) compareExposedPorts(current []api.ExposedPort, desired []config.ExposedPortConfig) []string {
	var changes []string

	// Check for count changes first
	if len(current) != len(desired) {
		changes = append(changes, fmt.Sprintf("ExposedPorts count: %d -> %d", len(current), len(desired)))
	}

	// Build maps by targetPort for comparison
	currentByPort := make(map[int32]api.ExposedPort)
	for _, port := range current {
		currentByPort[int32(port.TargetPort)] = port
	}

	desiredByPort := make(map[int32]config.ExposedPortConfig)
	for _, port := range desired {
		desiredByPort[port.TargetPort] = port
	}

	// Check for added and changed ports
	for _, desiredPort := range desired {
		currentPort, exists := currentByPort[desiredPort.TargetPort]
		if !exists {
			changes = append(changes, fmt.Sprintf("ExposedPort add: targetPort=%d", desiredPort.TargetPort))
			continue
		}

		// Compare fields
		prefix := fmt.Sprintf("ExposedPort[%d]", desiredPort.TargetPort)

		// LoadBalancerPort
		currentLBPort := int32(0)
		currentHasLB := false
		if !currentPort.LoadBalancerPort.IsNull() {
			currentLBPort = int32(currentPort.LoadBalancerPort.Value)
			currentHasLB = true
		}
		desiredLBPort := int32(0)
		desiredHasLB := false
		if desiredPort.LoadBalancerPort != nil {
			desiredLBPort = *desiredPort.LoadBalancerPort
			desiredHasLB = true
		}

		if currentHasLB && desiredHasLB && currentLBPort != desiredLBPort {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: %d -> %d", prefix, currentLBPort, desiredLBPort))
		} else if currentHasLB && !desiredHasLB {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: %d -> (unset)", prefix, currentLBPort))
		} else if !currentHasLB && desiredHasLB {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: (unset) -> %d", prefix, desiredLBPort))
		}

		// UseLetsEncrypt
		if currentPort.UseLetsEncrypt != desiredPort.UseLetsEncrypt {
			changes = append(changes, fmt.Sprintf("%s UseLetsEncrypt: %t -> %t", prefix, currentPort.UseLetsEncrypt, desiredPort.UseLetsEncrypt))
		}

		// Host
		if !stringSlicesEqual(currentPort.Host, desiredPort.Host) {
			changes = append(changes, fmt.Sprintf("%s Host: %v -> %v", prefix, currentPort.Host, desiredPort.Host))
		}

		// HealthCheck
		currentHasHC := !currentPort.HealthCheck.IsNull()
		desiredHasHC := desiredPort.HealthCheck != nil

		if currentHasHC && desiredHasHC {
			currentHC := currentPort.HealthCheck.Value
			desiredHC := desiredPort.HealthCheck
			if currentHC.Path != desiredHC.Path {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.Path: %s -> %s", prefix, currentHC.Path, desiredHC.Path))
			}
			if currentHC.IntervalSeconds != desiredHC.IntervalSeconds {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.IntervalSeconds: %d -> %d", prefix, currentHC.IntervalSeconds, desiredHC.IntervalSeconds))
			}
			if currentHC.TimeoutSeconds != desiredHC.TimeoutSeconds {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.TimeoutSeconds: %d -> %d", prefix, currentHC.TimeoutSeconds, desiredHC.TimeoutSeconds))
			}
		} else if currentHasHC && !desiredHasHC {
			changes = append(changes, fmt.Sprintf("%s HealthCheck: (set) -> (unset)", prefix))
		} else if !currentHasHC && desiredHasHC {
			changes = append(changes, fmt.Sprintf("%s HealthCheck: (unset) -> (set)", prefix))
		}
	}

	// Check for removed ports
	for _, currentPort := range current {
		if _, exists := desiredByPort[int32(currentPort.TargetPort)]; !exists {
			changes = append(changes, fmt.Sprintf("ExposedPort remove: targetPort=%d", currentPort.TargetPort))
		}
	}

	return changes
}

// updateSecretEnvVersions updates the state with secret env versions from config
func (p *Provisioner) updateSecretEnvVersions(appCfg *config.ApplicationConfig) bool {
	modified := false
	for _, env := range appCfg.Spec.Env {
		if env.Secret && env.SecretVersion != nil {
			storedVersion := p.state.GetSecretEnvVersion(appCfg.Name, env.Key)
			if storedVersion == nil || *storedVersion != *env.SecretVersion {
				p.state.SetSecretEnvVersion(appCfg.Name, env.Key, env.SecretVersion)
				modified = true
			}
		}
	}
	return modified
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
			return uuid.UUID{}, wrapAPIError(err, "failed to list clusters")
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
		return wrapAPIError(err, "failed to create application")
	}

	appID := createResp.Application.ApplicationID
	log.Printf("Created application %q with ID %s", appCfg.Name, uuid.UUID(appID))

	// Create the version (using image from config for new applications)
	versionReq := p.buildCreateVersionRequest(&appCfg.Spec)
	versionResp, err := p.client.CreateApplicationVersion(ctx, versionReq, api.CreateApplicationVersionParams{
		ApplicationID: appID,
	})
	if err != nil {
		return wrapAPIError(err, "failed to create version")
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
			return wrapAPIError(err, "failed to activate version")
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
		return wrapAPIError(err, "failed to get latest version")
	}

	// Create the new version (merge with existing settings)
	versionReq := p.buildCreateVersionRequestWithBase(&appCfg.Spec, latestVersion)
	versionResp, err := p.client.CreateApplicationVersion(ctx, versionReq, api.CreateApplicationVersionParams{
		ApplicationID: existing.ApplicationID,
	})
	if err != nil {
		return wrapAPIError(err, "failed to create version")
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
			return wrapAPIError(err, "failed to activate version")
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

// wrapAPIError wraps an API error with additional context, including response body if available
func wrapAPIError(err error, message string) error {
	if err == nil {
		return nil
	}

	// Try to extract the response body from DecodeBodyError
	var decodeErr *ogenerrors.DecodeBodyError
	if errors.As(err, &decodeErr) && len(decodeErr.Body) > 0 {
		return fmt.Errorf("%s: %w\nResponse body: %s", message, err, string(decodeErr.Body))
	}

	return fmt.Errorf("%s: %w", message, err)
}

// ListVersions returns all versions for an application
func (p *Provisioner) ListVersions(ctx context.Context, clusterName, appName string) (*VersionList, error) {
	// Resolve cluster name to ID
	clusterID, err := p.resolveClusterID(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cluster: %w", err)
	}

	// Find the application
	app, err := p.findApplicationByName(ctx, clusterID, appName)
	if err != nil {
		return nil, err
	}

	// Get active version
	activeVersion := 0
	if v, ok := app.ActiveVersion.Get(); ok {
		activeVersion = int(v)
	}

	// List all versions
	var allVersions []api.ApplicationVersionDeploymentStatus
	var cursor api.OptApplicationVersionNumber

	for {
		resp, err := p.client.ListApplicationVersions(ctx, api.ListApplicationVersionsParams{
			ApplicationID: app.ApplicationID,
			MaxItems:      30,
			Cursor:        cursor,
		})
		if err != nil {
			return nil, wrapAPIError(err, "failed to list versions")
		}

		allVersions = append(allVersions, resp.Versions...)

		if resp.NextCursor.Set {
			cursor = resp.NextCursor
		} else {
			break
		}
	}

	// Build result
	result := &VersionList{
		ApplicationName: appName,
		ApplicationID:   uuid.UUID(app.ApplicationID).String(),
		ActiveVersion:   activeVersion,
	}

	latestVersion := 0
	for _, v := range allVersions {
		versionNum := int(v.Version)
		if versionNum > latestVersion {
			latestVersion = versionNum
		}

		result.Versions = append(result.Versions, VersionInfo{
			Version:     versionNum,
			Image:       v.Image,
			Created:     time.Unix(int64(v.Created), 0),
			ActiveNodes: v.ActiveNodeCount,
			IsActive:    versionNum == activeVersion,
		})
	}
	result.LatestVersion = latestVersion

	return result, nil
}

// GetVersionDiff compares two versions and returns differences
func (p *Provisioner) GetVersionDiff(ctx context.Context, clusterName, appName string, fromVersion, toVersion int) (*VersionDiff, error) {
	// Resolve cluster name to ID
	clusterID, err := p.resolveClusterID(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cluster: %w", err)
	}

	// Find the application
	app, err := p.findApplicationByName(ctx, clusterID, appName)
	if err != nil {
		return nil, err
	}

	// Resolve version numbers (0 means active/latest)
	if fromVersion == 0 {
		if v, ok := app.ActiveVersion.Get(); ok {
			fromVersion = int(v)
		} else {
			return nil, fmt.Errorf("no active version exists for application %q", appName)
		}
	}

	if toVersion == 0 {
		latestVersion, err := p.getLatestVersion(ctx, app.ApplicationID)
		if err != nil {
			return nil, wrapAPIError(err, "failed to get latest version")
		}
		if latestVersion == nil {
			return nil, fmt.Errorf("no versions exist for application %q", appName)
		}
		toVersion = int(latestVersion.Version)
	}

	// Get full details of both versions
	fromVersionDetail, err := p.client.GetApplicationVersion(ctx, api.GetApplicationVersionParams{
		ApplicationID: app.ApplicationID,
		Version:       api.ApplicationVersionNumber(fromVersion),
	})
	if err != nil {
		return nil, wrapAPIError(err, fmt.Sprintf("failed to get version %d", fromVersion))
	}

	toVersionDetail, err := p.client.GetApplicationVersion(ctx, api.GetApplicationVersionParams{
		ApplicationID: app.ApplicationID,
		Version:       api.ApplicationVersionNumber(toVersion),
	})
	if err != nil {
		return nil, wrapAPIError(err, fmt.Sprintf("failed to get version %d", toVersion))
	}

	// Compare versions
	diff := &VersionDiff{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	from := &fromVersionDetail.ApplicationVersion
	to := &toVersionDetail.ApplicationVersion

	// Compare fields
	if from.CPU != to.CPU {
		diff.Changes = append(diff.Changes, fmt.Sprintf("CPU: %d -> %d", from.CPU, to.CPU))
	}
	if from.Memory != to.Memory {
		diff.Changes = append(diff.Changes, fmt.Sprintf("Memory: %d -> %d", from.Memory, to.Memory))
	}
	if from.ScalingMode != to.ScalingMode {
		diff.Changes = append(diff.Changes, fmt.Sprintf("ScalingMode: %s -> %s", from.ScalingMode, to.ScalingMode))
	}
	if from.Image != to.Image {
		diff.Changes = append(diff.Changes, fmt.Sprintf("Image: %s -> %s", from.Image, to.Image))
	}

	// Compare scaling parameters
	if fromVal, fromOk := from.FixedScale.Get(); fromOk {
		if toVal, toOk := to.FixedScale.Get(); toOk {
			if fromVal != toVal {
				diff.Changes = append(diff.Changes, fmt.Sprintf("FixedScale: %d -> %d", fromVal, toVal))
			}
		} else {
			diff.Changes = append(diff.Changes, fmt.Sprintf("FixedScale: %d -> (unset)", fromVal))
		}
	} else if toVal, toOk := to.FixedScale.Get(); toOk {
		diff.Changes = append(diff.Changes, fmt.Sprintf("FixedScale: (unset) -> %d", toVal))
	}

	if fromVal, fromOk := from.MinScale.Get(); fromOk {
		if toVal, toOk := to.MinScale.Get(); toOk {
			if fromVal != toVal {
				diff.Changes = append(diff.Changes, fmt.Sprintf("MinScale: %d -> %d", fromVal, toVal))
			}
		} else {
			diff.Changes = append(diff.Changes, fmt.Sprintf("MinScale: %d -> (unset)", fromVal))
		}
	} else if toVal, toOk := to.MinScale.Get(); toOk {
		diff.Changes = append(diff.Changes, fmt.Sprintf("MinScale: (unset) -> %d", toVal))
	}

	if fromVal, fromOk := from.MaxScale.Get(); fromOk {
		if toVal, toOk := to.MaxScale.Get(); toOk {
			if fromVal != toVal {
				diff.Changes = append(diff.Changes, fmt.Sprintf("MaxScale: %d -> %d", fromVal, toVal))
			}
		} else {
			diff.Changes = append(diff.Changes, fmt.Sprintf("MaxScale: %d -> (unset)", fromVal))
		}
	} else if toVal, toOk := to.MaxScale.Get(); toOk {
		diff.Changes = append(diff.Changes, fmt.Sprintf("MaxScale: (unset) -> %d", toVal))
	}

	// Compare Cmd
	if !stringSlicesEqual(from.Cmd, to.Cmd) {
		diff.Changes = append(diff.Changes, fmt.Sprintf("Cmd: %v -> %v", from.Cmd, to.Cmd))
	}

	// Compare registry credentials
	fromHasReg := !from.RegistryUsername.IsNull() && from.RegistryUsername.Value != ""
	toHasReg := !to.RegistryUsername.IsNull() && to.RegistryUsername.Value != ""

	if fromHasReg && toHasReg && from.RegistryUsername.Value != to.RegistryUsername.Value {
		diff.Changes = append(diff.Changes, fmt.Sprintf("RegistryUsername: %s -> %s", from.RegistryUsername.Value, to.RegistryUsername.Value))
	} else if fromHasReg && !toHasReg {
		diff.Changes = append(diff.Changes, fmt.Sprintf("RegistryUsername: %s -> (unset)", from.RegistryUsername.Value))
	} else if !fromHasReg && toHasReg {
		diff.Changes = append(diff.Changes, fmt.Sprintf("RegistryUsername: (unset) -> %s", to.RegistryUsername.Value))
	}

	// Check if registryPassword exists (cannot compare values)
	if fromHasReg || toHasReg {
		diff.HasRegistryPwd = true
	}

	// Compare env variables
	envDiff, hasSecrets := p.compareVersionEnv(from.Env, to.Env)
	diff.Changes = append(diff.Changes, envDiff...)
	diff.HasSecretEnv = hasSecrets

	// Compare exposed ports
	portChanges := p.compareVersionExposedPorts(from.ExposedPorts, to.ExposedPorts)
	diff.Changes = append(diff.Changes, portChanges...)

	return diff, nil
}

// compareVersionEnv compares environment variables between two versions
func (p *Provisioner) compareVersionEnv(from, to []api.ReadEnvironmentVariable) ([]string, bool) {
	var changes []string
	hasSecrets := false

	// Build maps for comparison
	fromByKey := make(map[string]api.ReadEnvironmentVariable)
	for _, env := range from {
		fromByKey[env.Key] = env
		if env.Secret {
			hasSecrets = true
		}
	}

	toByKey := make(map[string]api.ReadEnvironmentVariable)
	for _, env := range to {
		toByKey[env.Key] = env
		if env.Secret {
			hasSecrets = true
		}
	}

	// Check for added and changed env vars
	for _, toEnv := range to {
		fromEnv, exists := fromByKey[toEnv.Key]
		if !exists {
			// New env var
			if toEnv.Secret {
				changes = append(changes, fmt.Sprintf("Env add: %s (secret)", toEnv.Key))
			} else if !toEnv.Value.IsNull() {
				changes = append(changes, fmt.Sprintf("Env add: %s=%s", toEnv.Key, toEnv.Value.Value))
			} else {
				changes = append(changes, fmt.Sprintf("Env add: %s", toEnv.Key))
			}
			continue
		}

		// Env var exists in both, check for changes
		if toEnv.Secret || fromEnv.Secret {
			// Cannot compare secret values
			continue
		}

		// Compare non-secret values
		fromValue := ""
		if !fromEnv.Value.IsNull() {
			fromValue = fromEnv.Value.Value
		}
		toValue := ""
		if !toEnv.Value.IsNull() {
			toValue = toEnv.Value.Value
		}
		if fromValue != toValue {
			changes = append(changes, fmt.Sprintf("Env update: %s=%s -> %s", toEnv.Key, fromValue, toValue))
		}
	}

	// Check for removed env vars
	for _, fromEnv := range from {
		if _, exists := toByKey[fromEnv.Key]; !exists {
			if fromEnv.Secret {
				changes = append(changes, fmt.Sprintf("Env remove: %s (secret)", fromEnv.Key))
			} else {
				changes = append(changes, fmt.Sprintf("Env remove: %s", fromEnv.Key))
			}
		}
	}

	return changes, hasSecrets
}

// compareVersionExposedPorts compares exposed ports between two API versions
func (p *Provisioner) compareVersionExposedPorts(from, to []api.ExposedPort) []string {
	var changes []string

	// Check for count changes first
	if len(from) != len(to) {
		changes = append(changes, fmt.Sprintf("ExposedPorts count: %d -> %d", len(from), len(to)))
	}

	// Build maps by targetPort for comparison
	fromByPort := make(map[int32]api.ExposedPort)
	for _, port := range from {
		fromByPort[int32(port.TargetPort)] = port
	}

	toByPort := make(map[int32]api.ExposedPort)
	for _, port := range to {
		toByPort[int32(port.TargetPort)] = port
	}

	// Check for added and changed ports
	for _, toPort := range to {
		targetPort := int32(toPort.TargetPort)
		fromPort, exists := fromByPort[targetPort]
		if !exists {
			changes = append(changes, fmt.Sprintf("ExposedPort add: targetPort=%d", targetPort))
			continue
		}

		// Compare fields
		prefix := fmt.Sprintf("ExposedPort[%d]", targetPort)

		// LoadBalancerPort
		fromLBPort := int32(0)
		fromHasLB := false
		if !fromPort.LoadBalancerPort.IsNull() {
			fromLBPort = int32(fromPort.LoadBalancerPort.Value)
			fromHasLB = true
		}
		toLBPort := int32(0)
		toHasLB := false
		if !toPort.LoadBalancerPort.IsNull() {
			toLBPort = int32(toPort.LoadBalancerPort.Value)
			toHasLB = true
		}

		if fromHasLB && toHasLB && fromLBPort != toLBPort {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: %d -> %d", prefix, fromLBPort, toLBPort))
		} else if fromHasLB && !toHasLB {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: %d -> (unset)", prefix, fromLBPort))
		} else if !fromHasLB && toHasLB {
			changes = append(changes, fmt.Sprintf("%s LoadBalancerPort: (unset) -> %d", prefix, toLBPort))
		}

		// UseLetsEncrypt
		if fromPort.UseLetsEncrypt != toPort.UseLetsEncrypt {
			changes = append(changes, fmt.Sprintf("%s UseLetsEncrypt: %t -> %t", prefix, fromPort.UseLetsEncrypt, toPort.UseLetsEncrypt))
		}

		// Host
		if !stringSlicesEqual(fromPort.Host, toPort.Host) {
			changes = append(changes, fmt.Sprintf("%s Host: %v -> %v", prefix, fromPort.Host, toPort.Host))
		}

		// HealthCheck
		fromHasHC := !fromPort.HealthCheck.IsNull()
		toHasHC := !toPort.HealthCheck.IsNull()

		if fromHasHC && toHasHC {
			fromHC := fromPort.HealthCheck.Value
			toHC := toPort.HealthCheck.Value
			if fromHC.Path != toHC.Path {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.Path: %s -> %s", prefix, fromHC.Path, toHC.Path))
			}
			if fromHC.IntervalSeconds != toHC.IntervalSeconds {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.IntervalSeconds: %d -> %d", prefix, fromHC.IntervalSeconds, toHC.IntervalSeconds))
			}
			if fromHC.TimeoutSeconds != toHC.TimeoutSeconds {
				changes = append(changes, fmt.Sprintf("%s HealthCheck.TimeoutSeconds: %d -> %d", prefix, fromHC.TimeoutSeconds, toHC.TimeoutSeconds))
			}
		} else if fromHasHC && !toHasHC {
			changes = append(changes, fmt.Sprintf("%s HealthCheck: (set) -> (unset)", prefix))
		} else if !fromHasHC && toHasHC {
			changes = append(changes, fmt.Sprintf("%s HealthCheck: (unset) -> (set)", prefix))
		}
	}

	// Check for removed ports
	for _, fromPort := range from {
		if _, exists := toByPort[int32(fromPort.TargetPort)]; !exists {
			changes = append(changes, fmt.Sprintf("ExposedPort remove: targetPort=%d", fromPort.TargetPort))
		}
	}

	return changes
}

// ActivateVersion activates the specified version (0 means latest)
func (p *Provisioner) ActivateVersion(ctx context.Context, clusterName, appName string, version int) (int, error) {
	// Resolve cluster name to ID
	clusterID, err := p.resolveClusterID(ctx, clusterName)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve cluster: %w", err)
	}

	// Find the application
	app, err := p.findApplicationByName(ctx, clusterID, appName)
	if err != nil {
		return 0, err
	}

	// Resolve version number (0 means latest)
	if version == 0 {
		latestVersion, err := p.getLatestVersion(ctx, app.ApplicationID)
		if err != nil {
			return 0, wrapAPIError(err, "failed to get latest version")
		}
		if latestVersion == nil {
			return 0, fmt.Errorf("no versions exist for application %q", appName)
		}
		version = int(latestVersion.Version)
	}

	// Activate the version
	updateReq := &api.UpdateApplication{}
	updateReq.ActiveVersion.SetTo(int32(version))
	err = p.client.UpdateApplication(ctx, updateReq, api.UpdateApplicationParams{
		ApplicationID: app.ApplicationID,
	})
	if err != nil {
		return 0, wrapAPIError(err, "failed to activate version")
	}

	return version, nil
}

// findApplicationByName finds an application by name in the given cluster
func (p *Provisioner) findApplicationByName(ctx context.Context, clusterID uuid.UUID, appName string) (*api.ReadApplicationDetail, error) {
	apps, err := p.listAllApplications(ctx, clusterID)
	if err != nil {
		return nil, wrapAPIError(err, "failed to list applications")
	}

	for _, app := range apps {
		if app.Name == appName {
			return app, nil
		}
	}

	return nil, fmt.Errorf("application %q not found in cluster", appName)
}
