module github.com/plexsysio/ants-db

go 1.14

require (
	github.com/hsanjuan/ipfs-lite v1.1.14
	github.com/ipfs/go-datastore v0.5.0
	github.com/ipfs/go-ds-crdt v0.1.23-0.20211122112123-f4c55192c8e0
	github.com/ipfs/go-ipns v0.1.2
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/libp2p/go-libp2p v0.16.0
	github.com/libp2p/go-libp2p-core v0.11.0
	github.com/libp2p/go-libp2p-kad-dht v0.15.0
	github.com/libp2p/go-libp2p-pubsub v0.6.0
	github.com/libp2p/go-libp2p-record v0.1.3
	github.com/multiformats/go-multihash v0.1.0
	github.com/plexsysio/gkvstore v0.0.0-20211118085618-aa2812d0ec8d
	github.com/plexsysio/gkvstore-ipfsds v0.0.0-20211128070946-4089eab5b669
)

replace github.com/hsanjuan/ipfs-lite => github.com/plexsysio/ipfs-lite v1.1.22-0.20211128135214-3d31c70fbf56
