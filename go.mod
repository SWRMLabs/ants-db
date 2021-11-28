module github.com/SWRMLabs/ants-db

go 1.14

require (
	github.com/SWRMLabs/ss-ds-store v0.0.7
	github.com/SWRMLabs/ss-store v0.0.4
	github.com/hsanjuan/ipfs-lite v1.1.14
	github.com/ipfs/go-datastore v0.5.0
	github.com/ipfs/go-ds-crdt v0.1.22
	github.com/ipfs/go-ipns v0.1.2
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/libp2p/go-libp2p v0.15.1
	github.com/libp2p/go-libp2p-core v0.11.0
	github.com/libp2p/go-libp2p-kad-dht v0.13.1
	github.com/libp2p/go-libp2p-pubsub v0.6.0
	github.com/libp2p/go-libp2p-record v0.1.3
	github.com/multiformats/go-multihash v0.0.16
)

replace github.com/hsanjuan/ipfs-lite => ../ipfs-lite
