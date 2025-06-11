package contractsapi

import (
	"context"

	kwilClientType "github.com/kwilteam/kwil-db/core/client/types"
	"github.com/kwilteam/kwil-db/core/gatewayclient"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
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
		{input.Owner, input.RoleName, util.EthereumAddressesToStrings(input.Wallets)},
	}, opts...)
}

// RevokeRole revokes a role from multiple wallets.
// It calls the `revoke_roles` SQL action.
func (r *RoleManagement) RevokeRole(ctx context.Context, input types.RevokeRoleInput, opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	return r._client.Execute(ctx, "", "revoke_roles", [][]any{
		{input.Owner, input.RoleName, util.EthereumAddressesToStrings(input.Wallets)},
	}, opts...)
}

// AreMembersOf checks if the given wallets are members of a role.
// It calls the `are_members_of` SQL view action.
func (r *RoleManagement) AreMembersOf(ctx context.Context, input types.AreMembersOfInput) ([]types.RoleMembershipResult, error) {
	// Perform view call
	result, err := r._client.Call(ctx, "", "are_members_of", []any{
		input.Owner,
		input.RoleName,
		util.EthereumAddressesToStrings(input.Wallets),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if result.Error != nil {
		return nil, errors.New(*result.Error)
	}

	type membershipRaw struct {
		Wallet   string `json:"wallet"`
		IsMember bool   `json:"is_member"`
	}
	raw, err := DecodeCallResult[membershipRaw](result.QueryResult)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out := make([]types.RoleMembershipResult, len(raw))
	for i, r := range raw {
		addr, err := util.NewEthereumAddressFromString(r.Wallet)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		out[i] = types.RoleMembershipResult{Wallet: addr, IsMember: r.IsMember}
	}
	return out, nil
}

// ListRoleMembers lists the members of a role with pagination.
// It calls the `list_role_members` SQL view action.
func (r *RoleManagement) ListRoleMembers(ctx context.Context, input types.ListRoleMembersInput) ([]types.RoleMember, error) {
	// The SQL action enforces sensible defaults; pass provided limit/offset directly (can be zero).
	result, err := r._client.Call(ctx, "", "list_role_members", []any{
		input.Owner,
		input.RoleName,
		input.Limit,
		input.Offset,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if result.Error != nil {
		return nil, errors.New(*result.Error)
	}

	type memberRaw struct {
		Wallet    string `json:"wallet"`
		GrantedAt int64  `json:"granted_at"`
		GrantedBy string `json:"granted_by"`
	}
	raws, err := DecodeCallResult[memberRaw](result.QueryResult)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out := make([]types.RoleMember, len(raws))
	for i, m := range raws {
		wallet, err := util.NewEthereumAddressFromString(m.Wallet)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		out[i] = types.RoleMember{
			Wallet:    wallet,
			GrantedAt: m.GrantedAt,
			GrantedBy: m.GrantedBy,
		}
	}
	return out, nil
}
