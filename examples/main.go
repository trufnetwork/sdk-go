package main

import (
	"context"
	"fmt"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func main() {
	ctx := context.Background()
	streamId := util.GenerateStreamId("test")
	pk, _ := crypto.Secp256k1PrivateKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	signer := &auth.EthPersonalSigner{Key: *pk}
	tnClient, _ := tnclient.NewClient(
		ctx,
		"http://localhost:8484",
		tnclient.WithSigner(signer))
	fmt.Println("streamId: ", streamId)
	txHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	if err != nil {
		panic(err)
	}

	fmt.Println("txHash: ", txHash)
}
