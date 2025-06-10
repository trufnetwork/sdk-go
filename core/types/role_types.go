package types

import (
	"context"

	// Using the node/types for Kwil's Hash type for consistency
	kwilClientType "github.com/kwilteam/kwil-db/core/client/types" // for TxOpt
	"github.com/kwilteam/kwil-db/node/types"
)

// GrantRoleInput represents the input for granting a role to multiple wallets.
type GrantRoleInput struct {
	Owner    string
	RoleName string
	Wallets  []string // Changed to array of strings
}

// RevokeRoleInput represents the input for revoking a role from multiple wallets.
type RevokeRoleInput struct {
	Owner    string
	RoleName string
	Wallets  []string // Changed to array of strings
}

// IRoleManagement defines the interface for interacting with the role-based access control system.
type IRoleManagement interface {
	// GrantRole grants a specified role to multiple wallets.
	GrantRole(ctx context.Context, input GrantRoleInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
	// RevokeRole revokes a specified role from multiple wallets.
	RevokeRole(ctx context.Context, input RevokeRoleInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
}
