package contractsapi

import (
	"context"
	kwilClientType "github.com/kwilteam/kwil-db/core/client/types"
	"github.com/kwilteam/kwil-db/core/gatewayclient"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// RoleManagement provides methods to interact with the role-based access control system.
type RoleManagement struct {
	_client *gatewayclient.GatewayClient
}

var _ types.IRoleManagement = (*RoleManagement)(nil) // Ensure it implements the interface

// NewRoleManagementOptions defines options for creating a new RoleManagement instance.
type NewRoleManagementOptions struct {
	Client *gatewayclient.GatewayClient
}

// LoadRoleManagementActions creates a new RoleManagement instance.
func LoadRoleManagementActions(options NewRoleManagementOptions) (types.IRoleManagement, error) {
	if options.Client == nil {
		return nil, errors.New("Kwil client is required")
	}
	return &RoleManagement{
		_client: options.Client,
	}, nil
}

// GrantRole grants a role to multiple wallets.
// It calls the `grant_roles` SQL action.
func (r *RoleManagement) GrantRole(ctx context.Context, input types.GrantRoleInput, opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	return r._client.Execute(ctx, "", "grant_roles", [][]any{
		{input.Owner, input.RoleName, input.Wallets}, // Use input.Wallets directly
	}, opts...)
}

// RevokeRole revokes a role from multiple wallets.
// It calls the `revoke_roles` SQL action.
func (r *RoleManagement) RevokeRole(ctx context.Context, input types.RevokeRoleInput, opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	return r._client.Execute(ctx, "", "revoke_roles", [][]any{
		{input.Owner, input.RoleName, input.Wallets}, // Use input.Wallets directly
	}, opts...)
}
