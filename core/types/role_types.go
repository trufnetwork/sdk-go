package types

import (
	"context"

	kwilClientType "github.com/kwilteam/kwil-db/core/client/types" // for TxOpt
	"github.com/kwilteam/kwil-db/node/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// GrantRoleInput represents the input for granting a role to multiple wallets.
type GrantRoleInput struct {
	Owner    string
	RoleName string
	Wallets  []util.EthereumAddress
}

// RevokeRoleInput represents the input for revoking a role from multiple wallets.
type RevokeRoleInput struct {
	Owner    string
	RoleName string
	Wallets  []util.EthereumAddress
}

// AreMembersOfInput represents the input for checking if wallets are members of a role.
// It supports batch checking of multiple wallets in a single call.
type AreMembersOfInput struct {
	Owner    string
	RoleName string
	Wallets  []util.EthereumAddress
}

// ListRoleMembersInput represents the input for listing members of a role with pagination.
type ListRoleMembersInput struct {
	Owner    string
	RoleName string
	Limit    int
	Offset   int
}

// RoleMembershipResult represents the result of an AreMembersOf query.
// Wallet holds the queried wallet address, and IsMember indicates membership status.
type RoleMembershipResult struct {
	Wallet   util.EthereumAddress `json:"wallet"`
	IsMember bool                 `json:"is_member"`
}

// RoleMember represents a member of a role returned by list_role_members.
type RoleMember struct {
	Wallet    util.EthereumAddress `json:"wallet"`
	GrantedAt int64                `json:"granted_at"`
	GrantedBy string               `json:"granted_by"`
}

// IRoleManagement defines the interface for interacting with the role-based access control system.
type IRoleManagement interface {
	// GrantRole grants a specified role to multiple wallets.
	GrantRole(ctx context.Context, input GrantRoleInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
	// RevokeRole revokes a specified role from multiple wallets.
	RevokeRole(ctx context.Context, input RevokeRoleInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
	// AreMembersOf checks if one or more wallets are members of a specified role.
	AreMembersOf(ctx context.Context, input AreMembersOfInput) ([]RoleMembershipResult, error)
	// ListRoleMembers returns the members of a role with pagination.
	ListRoleMembers(ctx context.Context, input ListRoleMembersInput) ([]RoleMember, error)
}
