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
	tnClient, err := tnclient.NewClient(
		ctx,
		"http://localhost:8484",
		tnclient.WithSigner(signer))
	if err != nil {
		panic(err)
	}
	fmt.Println("streamId: ", streamId)
	listStreams, err := tnClient.ListStreams(ctx, types.ListStreamsInput{})
	if err != nil {
		panic(err)
	}

	fmt.Println("listStreams: ", listStreams)
}
