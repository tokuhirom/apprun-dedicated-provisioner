package provisioner

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
)

// ClusterAction represents a planned action for cluster settings
type ClusterAction struct {
	Action  ActionType
	Changes []string
}

// planClusterChanges compares current cluster settings with desired and returns planned changes
func (p *Provisioner) planClusterChanges(ctx context.Context, clusterID uuid.UUID, desired *config.ClusterSettings) (*ClusterAction, error) {
	if desired == nil {
		return &ClusterAction{Action: ActionNoop}, nil
	}

	// Get current cluster settings
	resp, err := p.client.GetCluster(ctx, api.GetClusterParams{
		ClusterID: api.ClusterID(clusterID),
	})
	if err != nil {
		return nil, wrapAPIError(err, "failed to get cluster")
	}

	current := resp.Cluster
	action := &ClusterAction{Action: ActionNoop}
	var changes []string

	// Compare LetsEncryptEmail
	// Note: API only returns HasLetsEncryptEmail (bool), not the actual email value
	// We can only detect: unset -> set, or set -> unset
	// We cannot detect changes from one email to another
	hasCurrentEmail := current.HasLetsEncryptEmail
	hasDesiredEmail := desired.LetsEncryptEmail != nil && *desired.LetsEncryptEmail != ""

	if !hasCurrentEmail && hasDesiredEmail {
		changes = append(changes, fmt.Sprintf("LetsEncryptEmail: (unset) -> %s", *desired.LetsEncryptEmail))
	} else if hasCurrentEmail && !hasDesiredEmail {
		changes = append(changes, "LetsEncryptEmail: (set) -> (unset)")
	} else if hasCurrentEmail && hasDesiredEmail {
		// Both are set, but we cannot compare values - always update to ensure desired state
		changes = append(changes, fmt.Sprintf("LetsEncryptEmail: (set) -> %s (value comparison not possible)", *desired.LetsEncryptEmail))
	}

	// Compare ServicePrincipalID
	if current.ServicePrincipalID != desired.ServicePrincipalID {
		changes = append(changes, fmt.Sprintf("ServicePrincipalID: %s -> %s", current.ServicePrincipalID, desired.ServicePrincipalID))
	}

	if len(changes) > 0 {
		action.Action = ActionUpdate
		action.Changes = changes
	}

	return action, nil
}

// applyClusterChanges applies cluster settings changes
func (p *Provisioner) applyClusterChanges(ctx context.Context, clusterID uuid.UUID, desired *config.ClusterSettings) error {
	if desired == nil {
		return nil
	}

	req := &api.UpdateCluster{
		ServicePrincipalID: desired.ServicePrincipalID,
	}

	if desired.LetsEncryptEmail != nil {
		req.LetsEncryptEmail.SetTo(*desired.LetsEncryptEmail)
	}

	err := p.client.UpdateCluster(ctx, req, api.UpdateClusterParams{
		ClusterID: api.ClusterID(clusterID),
	})
	if err != nil {
		return wrapAPIError(err, "failed to update cluster")
	}

	return nil
}
