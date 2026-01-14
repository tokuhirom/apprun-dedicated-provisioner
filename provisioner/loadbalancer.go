package provisioner

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
)

// LBActionType represents the type of action for a LoadBalancer
type LBActionType string

const (
	LBActionCreate   LBActionType = "create"
	LBActionDelete   LBActionType = "delete"
	LBActionRecreate LBActionType = "recreate"
	LBActionNoop     LBActionType = "noop"
)

// LBAction represents a planned action for a LoadBalancer
type LBAction struct {
	Action  LBActionType
	Name    string
	ASGName string
	Changes []string
	// For delete/recreate, we need the existing LB ID and ASG ID
	ExistingID *api.LoadBalancerID
	ASGID      *api.AutoScalingGroupID
}

// planLBChanges compares current LBs with desired and returns planned changes
func (p *Provisioner) planLBChanges(ctx context.Context, clusterID uuid.UUID, desired []config.LoadBalancerConfig, currentASGs []api.ReadAutoScalingGroupDetail) ([]LBAction, error) {
	// Build map of ASG names to IDs
	asgNameToID := make(map[string]api.AutoScalingGroupID)
	for _, asg := range currentASGs {
		asgNameToID[asg.Name] = asg.AutoScalingGroupID
	}

	// Get current LBs for all ASGs
	currentLBs := make(map[string]map[string]api.ReadLoadBalancerDetail) // asgName -> lbName -> LB
	for _, asg := range currentASGs {
		lbs, err := p.listAllLBs(ctx, clusterID, asg.AutoScalingGroupID)
		if err != nil {
			return nil, err
		}
		currentLBs[asg.Name] = make(map[string]api.ReadLoadBalancerDetail)
		for _, lb := range lbs {
			currentLBs[asg.Name][lb.Name] = lb
		}
	}

	var actions []LBAction

	// Check each desired LB
	desiredLBs := make(map[string]map[string]bool) // asgName -> lbName -> exists
	for _, desiredLB := range desired {
		if desiredLBs[desiredLB.AutoScalingGroupName] == nil {
			desiredLBs[desiredLB.AutoScalingGroupName] = make(map[string]bool)
		}
		desiredLBs[desiredLB.AutoScalingGroupName][desiredLB.Name] = true

		asgID, asgExists := asgNameToID[desiredLB.AutoScalingGroupName]
		if !asgExists {
			// ASG doesn't exist yet - LB will be created after ASG
			actions = append(actions, LBAction{
				Action:  LBActionCreate,
				Name:    desiredLB.Name,
				ASGName: desiredLB.AutoScalingGroupName,
				Changes: describeLBConfig(desiredLB),
			})
			continue
		}

		asgLBs := currentLBs[desiredLB.AutoScalingGroupName]
		current, exists := asgLBs[desiredLB.Name]
		if !exists {
			// LB doesn't exist, create it
			actions = append(actions, LBAction{
				Action:  LBActionCreate,
				Name:    desiredLB.Name,
				ASGName: desiredLB.AutoScalingGroupName,
				Changes: describeLBConfig(desiredLB),
				ASGID:   &asgID,
			})
		} else {
			// LB exists, check if settings differ
			changes := compareLB(current, desiredLB)
			if len(changes) > 0 {
				// Settings differ, need to recreate (no update API)
				lbID := current.LoadBalancerID
				actions = append(actions, LBAction{
					Action:     LBActionRecreate,
					Name:       desiredLB.Name,
					ASGName:    desiredLB.AutoScalingGroupName,
					Changes:    changes,
					ExistingID: &lbID,
					ASGID:      &asgID,
				})
			} else {
				actions = append(actions, LBAction{
					Action:  LBActionNoop,
					Name:    desiredLB.Name,
					ASGName: desiredLB.AutoScalingGroupName,
				})
			}
		}
	}

	// Check for LBs to delete (exist in current but not in desired)
	for asgName, lbMap := range currentLBs {
		asgID := asgNameToID[asgName]
		for lbName, lb := range lbMap {
			if desiredLBs[asgName] == nil || !desiredLBs[asgName][lbName] {
				lbID := lb.LoadBalancerID
				actions = append(actions, LBAction{
					Action:     LBActionDelete,
					Name:       lbName,
					ASGName:    asgName,
					Changes:    []string{"LB will be deleted"},
					ExistingID: &lbID,
					ASGID:      &asgID,
				})
			}
		}
	}

	return actions, nil
}

// listAllLBs retrieves all LBs for an ASG (handling pagination)
func (p *Provisioner) listAllLBs(ctx context.Context, clusterID uuid.UUID, asgID api.AutoScalingGroupID) ([]api.ReadLoadBalancerDetail, error) {
	var allLBs []api.ReadLoadBalancerDetail

	params := api.ListLoadBalancersParams{
		ClusterID:          api.ClusterID(clusterID),
		AutoScalingGroupID: asgID,
		MaxItems:           30,
	}

	for {
		resp, err := p.client.ListLoadBalancers(ctx, params)
		if err != nil {
			return nil, wrapAPIError(err, "failed to list load balancers")
		}

		// ListLoadBalancersResponse returns ReadLoadBalancerSummary, we need to get full details
		for _, summary := range resp.LoadBalancers {
			detail, err := p.client.GetLoadBalancer(ctx, api.GetLoadBalancerParams{
				ClusterID:          api.ClusterID(clusterID),
				AutoScalingGroupID: asgID,
				LoadBalancerID:     summary.LoadBalancerID,
			})
			if err != nil {
				return nil, wrapAPIError(err, fmt.Sprintf("failed to get load balancer %s", summary.Name))
			}
			allLBs = append(allLBs, detail.LoadBalancer)
		}

		if !resp.NextCursor.Set {
			break
		}
		params.Cursor = resp.NextCursor
	}

	return allLBs, nil
}

// compareLB compares current LB with desired config and returns differences
func compareLB(current api.ReadLoadBalancerDetail, desired config.LoadBalancerConfig) []string {
	var changes []string

	if current.ServiceClassPath != desired.ServiceClassPath {
		changes = append(changes, fmt.Sprintf("ServiceClassPath: %s -> %s", current.ServiceClassPath, desired.ServiceClassPath))
	}

	// Compare NameServers
	if !compareLBNameServers(current.NameServers, desired.NameServers) {
		changes = append(changes, fmt.Sprintf("NameServers: %v -> %v", current.NameServers, desired.NameServers))
	}

	// Compare Interfaces
	interfaceChanges := compareLBInterfaces(current.Interfaces, desired.Interfaces)
	changes = append(changes, interfaceChanges...)

	return changes
}

// compareLBNameServers compares two slices of name servers
func compareLBNameServers(current []api.IPv4, desired []string) bool {
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

// compareLBInterfaces compares interface configurations
func compareLBInterfaces(current []api.LoadBalancerInterface, desired []config.LBInterfaceConfig) []string {
	var changes []string

	if len(current) != len(desired) {
		changes = append(changes, fmt.Sprintf("Interfaces count: %d -> %d", len(current), len(desired)))
		return changes
	}

	// Build maps by interface index
	currentByIdx := make(map[int16]api.LoadBalancerInterface)
	desiredByIdx := make(map[int16]config.LBInterfaceConfig)

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

		// Compare Vip
		currentVip := ""
		if currentIface.Vip.Set {
			currentVip = currentIface.Vip.Value
		}
		desiredVip := ""
		if desiredIface.Vip != nil {
			desiredVip = *desiredIface.Vip
		}
		if currentVip != desiredVip {
			changes = append(changes, fmt.Sprintf("Interface[%d].Vip: %s -> %s", idx, currentVip, desiredVip))
		}

		// Compare VirtualRouterID
		currentVRID := int16(0)
		if currentIface.VirtualRouterID.Set {
			currentVRID = currentIface.VirtualRouterID.Value
		}
		desiredVRID := int16(0)
		if desiredIface.VirtualRouterID != nil {
			desiredVRID = *desiredIface.VirtualRouterID
		}
		if currentVRID != desiredVRID {
			changes = append(changes, fmt.Sprintf("Interface[%d].VirtualRouterID: %d -> %d", idx, currentVRID, desiredVRID))
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
		if !compareLBIPPools(currentIface.IpPool, desiredIface.IpPool) {
			changes = append(changes, fmt.Sprintf("Interface[%d].IpPool: changed", idx))
		}
	}

	return changes
}

// compareLBIPPools compares two IP pool configurations
func compareLBIPPools(current []api.IpRange, desired []config.IpRangeConfig) bool {
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

// describeLBConfig returns a description of LB configuration for plan output
func describeLBConfig(cfg config.LoadBalancerConfig) []string {
	return []string{
		fmt.Sprintf("AutoScalingGroup: %s", cfg.AutoScalingGroupName),
		fmt.Sprintf("ServiceClassPath: %s", cfg.ServiceClassPath),
		fmt.Sprintf("NameServers: %v", cfg.NameServers),
		fmt.Sprintf("Interfaces: %d configured", len(cfg.Interfaces)),
	}
}

// applyLBChanges applies the planned LB changes
func (p *Provisioner) applyLBChanges(ctx context.Context, clusterID uuid.UUID, actions []LBAction, desired []config.LoadBalancerConfig, asgNameToID map[string]api.AutoScalingGroupID) error {
	// Build map of desired configs by ASG name and LB name
	desiredByKey := make(map[string]config.LoadBalancerConfig) // "asgName/lbName" -> config
	for _, cfg := range desired {
		key := cfg.AutoScalingGroupName + "/" + cfg.Name
		desiredByKey[key] = cfg
	}

	// Process actions in order: delete first, then create
	// This handles recreate scenarios

	// First, delete LBs that need to be removed or recreated
	for _, action := range actions {
		if action.Action == LBActionDelete || action.Action == LBActionRecreate {
			if action.ExistingID == nil || action.ASGID == nil {
				return fmt.Errorf("cannot delete LB %s: missing ID", action.Name)
			}
			fmt.Printf("Deleting LB: %s (ASG: %s)\n", action.Name, action.ASGName)
			err := p.client.DeleteLoadBalancer(ctx, api.DeleteLoadBalancerParams{
				ClusterID:          api.ClusterID(clusterID),
				AutoScalingGroupID: *action.ASGID,
				LoadBalancerID:     *action.ExistingID,
			})
			if err != nil {
				return wrapAPIError(err, fmt.Sprintf("failed to delete LB %s", action.Name))
			}
		}
	}

	// Then, create LBs that need to be created or recreated
	for _, action := range actions {
		if action.Action == LBActionCreate || action.Action == LBActionRecreate {
			key := action.ASGName + "/" + action.Name
			cfg, ok := desiredByKey[key]
			if !ok {
				return fmt.Errorf("cannot create LB %s: config not found", action.Name)
			}

			// Get ASG ID (might be newly created)
			asgID, ok := asgNameToID[action.ASGName]
			if !ok {
				return fmt.Errorf("cannot create LB %s: ASG %s not found", action.Name, action.ASGName)
			}

			fmt.Printf("Creating LB: %s (ASG: %s)\n", action.Name, action.ASGName)
			req := buildCreateLBRequest(cfg)
			_, err := p.client.CreateLoadBalancer(ctx, req, api.CreateLoadBalancerParams{
				ClusterID:          api.ClusterID(clusterID),
				AutoScalingGroupID: asgID,
			})
			if err != nil {
				return wrapAPIError(err, fmt.Sprintf("failed to create LB %s", action.Name))
			}
		}
	}

	return nil
}

// buildCreateLBRequest builds the API request from config
func buildCreateLBRequest(cfg config.LoadBalancerConfig) *api.CreateLoadBalancer {
	req := &api.CreateLoadBalancer{
		Name:             cfg.Name,
		ServiceClassPath: cfg.ServiceClassPath,
	}

	// Convert NameServers
	for _, ns := range cfg.NameServers {
		req.NameServers = append(req.NameServers, api.IPv4(ns))
	}

	// Convert Interfaces
	for _, iface := range cfg.Interfaces {
		apiIface := api.LoadBalancerInterface{
			InterfaceIndex: iface.InterfaceIndex,
			Upstream:       iface.Upstream,
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
		if iface.Vip != nil {
			apiIface.Vip.SetTo(*iface.Vip)
		}
		if iface.VirtualRouterID != nil {
			apiIface.VirtualRouterID.SetTo(*iface.VirtualRouterID)
		}
		if iface.PacketFilterID != nil {
			apiIface.PacketFilterID.SetTo(*iface.PacketFilterID)
		}

		req.Interfaces = append(req.Interfaces, apiIface)
	}

	return req
}
