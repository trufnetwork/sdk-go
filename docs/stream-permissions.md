# Stream Permissions in TN

The TRUF.NETWORK (TN) provides granular control over stream access and visibility. This document outlines the permission system, how to configure it, and best practices for securing your streams.

## Permission Types

TN supports two main types of permissions:

1. **Read Permissions**: Control who can read data from a stream.
2. **Compose Permissions**: Determine which streams can use this stream as a child in a composed stream.

## Visibility Settings

Streams can be set to one of two visibility states:

- **Public**: Accessible to all users.
- **Private**: Accessible only to specifically allowed wallets or streams.

## Managing Permissions

### Setting Stream Visibility

To set a stream's visibility:

```go
// Set read visibility
txHash, err := stream.SetReadVisibility(ctx, util.PrivateVisibility)
if err != nil {
    // Handle error
}

// Set compose visibility
txHash, err := stream.SetComposeVisibility(ctx, util.PublicVisibility)
if err != nil {
    // Handle error
}
```

### Allowing Specific Wallets to Read

For private streams, you can allow specific wallets to read data:

```go
txHash, err := stream.AllowReadWallet(ctx, readerAddress)
if err != nil {
    // Handle error
}
```

### Allowing Streams to Compose

You can allow specific streams to use your stream as a child in composition:

```go
txHash, err := stream.AllowComposeStream(ctx, composedStreamLocator)
if err != nil {
    // Handle error
}
```

### Revoking Permissions

To revoke previously granted permissions:

```go
// Revoke read permission
txHash, err := stream.DisableReadWallet(ctx, readerAddress)
if err != nil {
    // Handle error
}

// Revoke compose permission
txHash, err := stream.DisableComposeStream(ctx, composedStreamLocator)
if err != nil {
    // Handle error
}
```

## Checking Current Permissions

You can query the current permission settings:

```go
// Check read visibility
visibility, err := stream.GetReadVisibility(ctx)
if err != nil {
    // Handle error
}

// Get allowed read wallets
allowedWallets, err := stream.GetAllowedReadWallets(ctx)
if err != nil {
    // Handle error
}

// Get allowed compose streams
allowedStreams, err := stream.GetAllowedComposeStreams(ctx)
if err != nil {
    // Handle error
}
```

## Permission Scenarios

### Scenario 1: Public Read, Private Compose

This configuration allows anyone to read the stream data, but only specific streams can use it in composition.

```go
stream.SetReadVisibility(ctx, util.PublicVisibility)
stream.SetComposeVisibility(ctx, util.PrivateVisibility)
stream.AllowComposeStream(ctx, allowedStreamLocator)
```

### Scenario 2: Private Read, Public Compose

Only specific wallets can read the stream data, but any stream can use it in composition.

```go
stream.SetReadVisibility(ctx, util.PrivateVisibility)
stream.SetComposeVisibility(ctx, util.PublicVisibility)
stream.AllowReadWallet(ctx, allowedReaderAddress)
```

### Scenario 3: Fully Private

Both reading and composing are restricted to specifically allowed entities.

```go
stream.SetReadVisibility(ctx, util.PrivateVisibility)
stream.SetComposeVisibility(ctx, util.PrivateVisibility)
stream.AllowReadWallet(ctx, allowedReaderAddress)
stream.AllowComposeStream(ctx, allowedStreamLocator)
```

## Caveats and Considerations

- Changing permissions requires blockchain transactions. Always wait for transaction confirmation before assuming the change has taken effect.

## Network Writer Role for Stream Creation

To ensure the integrity and quality of data streams on the Truf Network, the creation of new streams (both primitive and composed) is a permissioned operation. This process is governed by a **`system:network_writer`** role within the network's role-based access control (RBAC) system.

Only wallets that are members of the `system:network_writer` role are authorized to deploy new streams using the SDK's `DeployStream` or `BatchDeployStreams` functions.

**How to get access:**

If you are a partner or data provider interested in deploying streams on the TRUF.NETWORK, please contact our team. We will guide you through the process of obtaining the necessary permissions by granting your wallet the `system:network_writer` role.

### SDK Role Management API (For TRUF.NETWORK Internal Use / Advanced Partners)

The SDK provides an API for advanced partners and internal use to programmatically interact with the role management system. These functions allow for granting, revoking, and checking role memberships.

-   `Client.LoadRoleManagementActions()`: Loads the `IRoleManagement` interface.
-   `IRoleManagement.GrantRole(ctx, GrantRoleInput, ...TxOpt)`: Grants a role to one or more wallets.
-   `IRoleManagement.RevokeRole(ctx, RevokeRoleInput, ...TxOpt)`: Revokes a role from one or more wallets.
-   `IRoleManagement.AreMembersOf(ctx, AreMembersOfInput)`: Checks if one or more wallets are members of a specific role.
-   `IRoleManagement.ListRoleMembers(ctx, ListRoleMembersInput)`: Lists current members of a role with optional pagination.

**Note:** For general stream creation, users should typically contact the TRUF.NETWORK team directly rather than attempting to manage the `system:network_writer` role themselves. The `system:network_writer` role is managed by `system:network_writers_manager`.

By leveraging these permission controls, you can create secure, flexible data streams that meet your specific needs while maintaining control over your valuable data within the TN ecosystem.