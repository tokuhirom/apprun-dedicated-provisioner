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
)

// ASGActionType represents the type of action for an ASG
type ASGActionType string

const (
	ASGActionCreate   ASGActionType = "create"
	ASGActionDelete   ASGActionType = "delete"
	ASGActionRecreate ASGActionType = "recreate"
	ASGActionNoop     ASGActionType = "noop"
	ASGActionSkip     ASGActionType = "skip" // exists but not in YAML, skip
)

// ASGAction represents a planned action for an ASG
type ASGAction struct {
	Action  ASGActionType
	Name    string
	Changes []string
	// For delete/recreate, we need the existing ASG ID
	ExistingID *api.AutoScalingGroupID
}

// planASGChanges compares current ASGs with desired and returns planned changes
func (p *Provisioner) planASGChanges(ctx context.Context, clusterID uuid.UUID, desired []config.AutoScalingGroupConfig) ([]ASGAction, error) {
	// Get current ASGs
	currentASGs, err := p.listAllASGs(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// Build map of current ASGs by name
	currentByName := make(map[string]api.ReadAutoScalingGroupDetail)
	for _, asg := range currentASGs {
		currentByName[asg.Name] = asg
	}

	var actions []ASGAction

	// Check each desired ASG
	desiredNames := make(map[string]bool)
	for _, desiredASG := range desired {
		desiredNames[desiredASG.Name] = true

		current, exists := currentByName[desiredASG.Name]
		if !exists {
			// ASG doesn't exist, create it
			actions = append(actions, ASGAction{
				Action:  ASGActionCreate,
				Name:    desiredASG.Name,
				Changes: describeASGConfig(desiredASG),
			})
		} else {
			// ASG exists, check if settings differ
			changes := compareASG(current, desiredASG)
			if len(changes) > 0 {
				// Settings differ, need to recreate (no update API)
				asgID := current.AutoScalingGroupID
				actions = append(actions, ASGAction{
					Action:     ASGActionRecreate,
					Name:       desiredASG.Name,
					Changes:    changes,
					ExistingID: &asgID,
				})
			} else {
				actions = append(actions, ASGAction{
					Action: ASGActionNoop,
					Name:   desiredASG.Name,
				})
			}
		}
	}

	// Check for ASGs not in YAML (skip instead of delete)
	for name := range currentByName {
		if !desiredNames[name] {
			actions = append(actions, ASGAction{
				Action:  ASGActionSkip,
				Name:    name,
				Changes: []string{"not in YAML, skipping"},
			})
		}
	}

	return actions, nil
}

// listAllASGs retrieves all ASGs for a cluster (handling pagination)
func (p *Provisioner) listAllASGs(ctx context.Context, clusterID uuid.UUID) ([]api.ReadAutoScalingGroupDetail, error) {
	var allASGs []api.ReadAutoScalingGroupDetail

	params := api.ListAutoScalingGroupsParams{
		ClusterID: api.ClusterID(clusterID),
		MaxItems:  30,
	}

	for {
		resp, err := p.client.ListAutoScalingGroups(ctx, params)
		if err != nil {
			return nil, wrapAPIError(err, "failed to list auto scaling groups")
		}

		allASGs = append(allASGs, resp.AutoScalingGroups...)

		if !resp.NextCursor.Set {
			break
		}
		params.Cursor = resp.NextCursor
	}

	return allASGs, nil
}

// compareASG compares current ASG with desired config and returns differences
func compareASG(current api.ReadAutoScalingGroupDetail, desired config.AutoScalingGroupConfig) []string {
	var changes []string

	if current.Zone != desired.Zone {
		changes = append(changes, fmt.Sprintf("Zone: %s -> %s", current.Zone, desired.Zone))
	}

	if current.WorkerServiceClassPath != desired.WorkerServiceClassPath {
		changes = append(changes, fmt.Sprintf("WorkerServiceClassPath: %s -> %s", current.WorkerServiceClassPath, desired.WorkerServiceClassPath))
	}

	if current.MinNodes != desired.MinNodes {
		changes = append(changes, fmt.Sprintf("MinNodes: %d -> %d", current.MinNodes, desired.MinNodes))
	}

	if current.MaxNodes != desired.MaxNodes {
		changes = append(changes, fmt.Sprintf("MaxNodes: %d -> %d", current.MaxNodes, desired.MaxNodes))
	}

	// Compare NameServers
	if !compareNameServers(current.NameServers, desired.NameServers) {
		changes = append(changes, fmt.Sprintf("NameServers: %v -> %v", current.NameServers, desired.NameServers))
	}

	// Compare Interfaces
	interfaceChanges := compareASGInterfaces(current.Interfaces, desired.Interfaces)
	changes = append(changes, interfaceChanges...)

	return changes
}

// compareNameServers compares two slices of name servers
func compareNameServers(current []api.IPv4, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	for i, c := range current {
		if string(c) != desired[i] {
			return false
		}
	}
	return true
}

// compareASGInterfaces compares interface configurations
func compareASGInterfaces(current []api.AutoScalingGroupNodeInterface, desired []config.ASGInterfaceConfig) []string {
	var changes []string

	if len(current) != len(desired) {
		changes = append(changes, fmt.Sprintf("Interfaces count: %d -> %d", len(current), len(desired)))
		return changes
	}

	// Build maps by interface index
	currentByIdx := make(map[int16]api.AutoScalingGroupNodeInterface)
	desiredByIdx := make(map[int16]config.ASGInterfaceConfig)

	for _, iface := range current {
		currentByIdx[iface.InterfaceIndex] = iface
	}
	for _, iface := range desired {
		desiredByIdx[iface.InterfaceIndex] = iface
	}

	for idx, desiredIface := range desiredByIdx {
		currentIface, exists := currentByIdx[idx]
		if !exists {
			changes = append(changes, fmt.Sprintf("Interface[%d]: new interface", idx))
			continue
		}

		if currentIface.Upstream != desiredIface.Upstream {
			changes = append(changes, fmt.Sprintf("Interface[%d].Upstream: %s -> %s", idx, currentIface.Upstream, desiredIface.Upstream))
		}

		if currentIface.ConnectsToLB != desiredIface.ConnectsToLB {
			changes = append(changes, fmt.Sprintf("Interface[%d].ConnectsToLB: %v -> %v", idx, currentIface.ConnectsToLB, desiredIface.ConnectsToLB))
		}

		// Compare NetmaskLen
		currentNetmask := int16(0)
		if currentIface.NetmaskLen.Set {
			currentNetmask = currentIface.NetmaskLen.Value
		}
		desiredNetmask := int16(0)
		if desiredIface.NetmaskLen != nil {
			desiredNetmask = *desiredIface.NetmaskLen
		}
		if currentNetmask != desiredNetmask {
			changes = append(changes, fmt.Sprintf("Interface[%d].NetmaskLen: %d -> %d", idx, currentNetmask, desiredNetmask))
		}

		// Compare DefaultGateway
		currentGW := ""
		if currentIface.DefaultGateway.Set {
			currentGW = currentIface.DefaultGateway.Value
		}
		desiredGW := ""
		if desiredIface.DefaultGateway != nil {
			desiredGW = *desiredIface.DefaultGateway
		}
		if currentGW != desiredGW {
			changes = append(changes, fmt.Sprintf("Interface[%d].DefaultGateway: %s -> %s", idx, currentGW, desiredGW))
		}

		// Compare PacketFilterID
		currentPF := ""
		if currentIface.PacketFilterID.Set {
			currentPF = currentIface.PacketFilterID.Value
		}
		desiredPF := ""
		if desiredIface.PacketFilterID != nil {
			desiredPF = *desiredIface.PacketFilterID
		}
		if currentPF != desiredPF {
			changes = append(changes, fmt.Sprintf("Interface[%d].PacketFilterID: %s -> %s", idx, currentPF, desiredPF))
		}

		// Compare IpPool
		if !compareIPPools(currentIface.IpPool, desiredIface.IpPool) {
			changes = append(changes, fmt.Sprintf("Interface[%d].IpPool: changed", idx))
		}
	}

	return changes
}

// compareIPPools compares two IP pool configurations
func compareIPPools(current []api.IpRange, desired []config.IpRangeConfig) bool {
	if len(current) != len(desired) {
		return false
	}
	for i, c := range current {
		if string(c.Start) != desired[i].Start || string(c.End) != desired[i].End {
			return false
		}
	}
	return true
}

// describeASGConfig returns a description of ASG configuration for plan output
func describeASGConfig(cfg config.AutoScalingGroupConfig) []string {
	return []string{
		fmt.Sprintf("Zone: %s", cfg.Zone),
		fmt.Sprintf("WorkerServiceClassPath: %s", cfg.WorkerServiceClassPath),
		fmt.Sprintf("MinNodes: %d, MaxNodes: %d", cfg.MinNodes, cfg.MaxNodes),
		fmt.Sprintf("NameServers: %v", cfg.NameServers),
		fmt.Sprintf("Interfaces: %d configured", len(cfg.Interfaces)),
	}
}

// applyASGChanges applies the planned ASG changes
func (p *Provisioner) applyASGChanges(ctx context.Context, clusterID uuid.UUID, actions []ASGAction, desired []config.AutoScalingGroupConfig) error {
	// Build map of desired configs by name
	desiredByName := make(map[string]config.AutoScalingGroupConfig)
	for _, cfg := range desired {
		desiredByName[cfg.Name] = cfg
	}

	// Process actions in order: delete first, then create
	// This handles recreate scenarios

	// First, delete ASGs that need to be removed or recreated
	for _, action := range actions {
		if action.Action == ASGActionDelete || action.Action == ASGActionRecreate {
			if action.ExistingID == nil {
				return fmt.Errorf("cannot delete ASG %s: missing ID", action.Name)
			}
			fmt.Printf("Deleting ASG: %s\n", action.Name)
			err := p.client.DeleteAutoScalingGroup(ctx, api.DeleteAutoScalingGroupParams{
				ClusterID:          api.ClusterID(clusterID),
				AutoScalingGroupID: *action.ExistingID,
			})
			if err != nil {
				return wrapAPIError(err, fmt.Sprintf("failed to delete ASG %s", action.Name))
			}
		}
	}

	// Then, create ASGs that need to be created or recreated
	for _, action := range actions {
		if action.Action == ASGActionCreate || action.Action == ASGActionRecreate {
			cfg, ok := desiredByName[action.Name]
			if !ok {
				return fmt.Errorf("cannot create ASG %s: config not found", action.Name)
			}

			fmt.Printf("Creating ASG: %s\n", action.Name)
			req := buildCreateASGRequest(cfg)
			_, err := p.client.CreateAutoScalingGroup(ctx, req, api.CreateAutoScalingGroupParams{
				ClusterID: api.ClusterID(clusterID),
			})
			if err != nil {
				return wrapAPIError(err, fmt.Sprintf("failed to create ASG %s", action.Name))
			}
		}
	}

	return nil
}

// buildCreateASGRequest builds the API request from config
func buildCreateASGRequest(cfg config.AutoScalingGroupConfig) *api.CreateAutoScalingGroup {
	req := &api.CreateAutoScalingGroup{
		Name:                   cfg.Name,
		Zone:                   cfg.Zone,
		WorkerServiceClassPath: cfg.WorkerServiceClassPath,
		MinNodes:               cfg.MinNodes,
		MaxNodes:               cfg.MaxNodes,
	}

	// Convert NameServers
	for _, ns := range cfg.NameServers {
		req.NameServers = append(req.NameServers, api.IPv4(ns))
	}

	// Convert Interfaces
	for _, iface := range cfg.Interfaces {
		apiIface := api.AutoScalingGroupNodeInterface{
			InterfaceIndex: iface.InterfaceIndex,
			Upstream:       iface.Upstream,
			ConnectsToLB:   iface.ConnectsToLB,
		}

		// Convert IpPool
		for _, ipRange := range iface.IpPool {
			apiIface.IpPool = append(apiIface.IpPool, api.IpRange{
				Start: api.IPv4(ipRange.Start),
				End:   api.IPv4(ipRange.End),
			})
		}

		// Set optional fields
		if iface.NetmaskLen != nil {
			apiIface.NetmaskLen.SetTo(*iface.NetmaskLen)
		}
		if iface.DefaultGateway != nil {
			apiIface.DefaultGateway.SetTo(*iface.DefaultGateway)
		}
		if iface.PacketFilterID != nil {
			apiIface.PacketFilterID.SetTo(*iface.PacketFilterID)
		}

		req.Interfaces = append(req.Interfaces, apiIface)
	}

	return req
}

// waitForASGDeletion polls until the ASG is deleted or timeout
func (p *Provisioner) waitForASGDeletion(ctx context.Context, clusterID uuid.UUID, asgID api.AutoScalingGroupID, asgName string) error {
	startTime := time.Now()
	pollInterval := 3 * time.Second
	timeout := 5 * time.Minute

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			return fmt.Errorf("timeout waiting for ASG %s deletion after %v", asgName, elapsed)
		}

		// Try to get the ASG
		_, err := p.client.GetAutoScalingGroup(ctx, api.GetAutoScalingGroupParams{
			ClusterID:          api.ClusterID(clusterID),
			AutoScalingGroupID: asgID,
		})

		if err != nil {
			// Check if it's a 404 error (ASG deleted)
			var secErr *ogenerrors.SecurityError
			if errors.As(err, &secErr) {
				// Security error means we can't access it
				return fmt.Errorf("failed to check ASG status: %w", err)
			}
			// Assume deleted if we get an error (typically 404)
			log.Printf("ASG %s deleted (elapsed: %.1fs)", asgName, elapsed.Seconds())
			return nil
		}

		log.Printf("Waiting for ASG %s deletion... (elapsed: %.1fs)", asgName, elapsed.Seconds())
		time.Sleep(pollInterval)
	}
}
