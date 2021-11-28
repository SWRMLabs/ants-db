# ants-db
Distributed KV store using go-ds-crdt and libp2p

AntsDB provides a simple KV interface over go-ds-crdt. The code is highly inspired
by the example in [ipfs-cluster/consesus](https://github.com/ipfs/ipfs-cluster/tree/master/consensus/crdt)

Its especially useful if you are already using the IPFS ecosystem. With minimal
code layer we can create this KV Store which can be used by apps.

## Install
`ants-db` works like a regular Go module:

```
> go get github.com/plexsysio/ants-db
```

Check tests for examples
