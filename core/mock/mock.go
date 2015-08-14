package coremock

import (
	"encoding/base64"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	blockservice "github.com/ipfs/go-ipfs/blockservice"
	commands "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	nsys "github.com/ipfs/go-ipfs/namesys"
	mocknet "github.com/ipfs/go-ipfs/p2p/net/mock"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	dht "github.com/ipfs/go-ipfs/routing/dht"
	ds2 "github.com/ipfs/go-ipfs/util/datastore2"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// effectively offline, only peer in its network
	return NewMockNodeOnNet(ctx, mocknet.New(ctx))
}

// TODO: shrink this function as much as possible, moving logic into NewNode
func NewMockNodeOnNet(ctx context.Context, mn mocknet.Mocknet) (*core.IpfsNode, error) {

	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return nil, err
	}
	p := ident.ID()

	pkb, err := ident.PrivateKey().Bytes()
	if err != nil {
		return nil, err
	}

	c := config.Config{
		Identity: config.Identity{
			PeerID:  p.String(),
			PrivKey: base64.StdEncoding.EncodeToString(pkb),
		},
	}

	r := &repo.Mock{
		C: c,
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
	}
	nd, err := core.NewNode(ctx, &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		return nil, err
	}

	nd.PeerHost, err = mn.AddPeer(ident.PrivateKey(), ident.Address())
	if err != nil {
		return nil, err
	}

	nd.Peerstore = nd.PeerHost.Peerstore()
	nd.Peerstore.AddPrivKey(p, ident.PrivateKey())
	nd.Peerstore.AddPubKey(p, ident.PublicKey())
	nd.Identity = p

	// Routing
	nd.Routing = dht.NewDHT(ctx, nd.PeerHost, nd.Repo.Datastore())

	// Bitswap
	bstore := blockstore.NewBlockstore(nd.Repo.Datastore())

	bsn := bsnet.NewFromIpfsHost(nd.PeerHost, nd.Routing)
	nd.Exchange = bitswap.New(ctx, p, bsn, bstore, true)

	bserv, err := blockservice.New(bstore, nd.Exchange)
	if err != nil {
		return nil, err
	}

	nd.DAG = mdag.NewDAGService(bserv)
	nd.Pinning = pin.NewPinner(nd.Repo.Datastore(), nd.DAG)

	// Namespace resolver
	nd.Namesys = nsys.NewNameSystem(nd.Routing)

	// Path resolver
	nd.Resolver = &path.Resolver{DAG: nd.DAG}

	return nd, nil
}

func MockCmdsCtx() (commands.Context, error) {
	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return commands.Context{}, err
	}
	p := ident.ID()

	conf := config.Config{
		Identity: config.Identity{
			PeerID: p.String(),
		},
	}

	r := &repo.Mock{
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
		C: conf,
	}

	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Repo: r,
	})

	return commands.Context{
		Online:     true,
		ConfigRoot: "/tmp/.mockipfsconfig",
		LoadConfig: func(path string) (*config.Config, error) {
			return &conf, nil
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}, nil
}
