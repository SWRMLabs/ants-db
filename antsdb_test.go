package antsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	ipns "github.com/ipfs/go-ipns"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	record "github.com/libp2p/go-libp2p-record"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	store "github.com/plexsysio/gkvstore"
)

func makeTestingHost(t *testing.T, opts ...Option) (*AntsDB, host.Host) {
	ctx, cancel := context.WithCancel(context.Background())
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	if err != nil {
		t.Fatal(err)
	}
	psub, err := pubsub.NewGossipSub(
		ctx,
		h,
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
	)
	if err != nil {
		h.Close()
		t.Fatal(err)
	}
	idht, err := dual.New(ctx, h,
		dual.DHTOption(
			dht.NamespacedValidator("pk", record.PublicKeyValidator{}),
			dht.NamespacedValidator("ipns", ipns.Validator{KeyBook: h.Peerstore()}),
			dht.Concurrency(10),
			dht.RoutingTableRefreshPeriod(200*time.Millisecond),
			dht.RoutingTableRefreshQueryTimeout(100*time.Millisecond),
		),
	)
	if err != nil {
		h.Close()
		t.Fatal(err)
	}
	rHost := routedhost.Wrap(h, idht)

	bs := syncds.MutexWrap(datastore.NewMapDatastore())

	ipfs, err := ipfslite.New(
		ctx,
		bs,
		rHost,
		idht,
		&ipfslite.Config{
			Offline: false,
		},
	)
	opts = append(opts,
		WithRebroadcastDuration(time.Second),
		WithOnCloseHook(func() {
			cancel()
			log.Info("Stopping host")
			h.Close()
			log.Info("Stopping DHT")
			idht.Close()
		}),
	)
	adb, err := New(
		ipfs,
		psub,
		bs,
		opts...,
	)
	if err != nil {
		h.Close()
		idht.Close()
		t.Fatal(err)
	}
	return adb, h
}

func connectHosts(t *testing.T, hosts ...host.Host) {
	for i, h1 := range hosts {
		rest := []host.Host{}
		for j, h2 := range hosts {
			if i != j {
				rest = append(rest, h2)
			}
		}
		for _, h2 := range rest {
			if h1.Network().Connectedness(h2.ID()) == network.Connected {
				t.Logf("Already connected %v : %v", h1.ID(), h2.ID())
				continue
			}
			t.Logf("Connecting %v to %v", h1.ID(), h2.ID())
			h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
			_, err := h1.Network().DialPeer(context.Background(), h2.ID())
			if err != nil {
				t.Fatal("Failed dialing peer ", err.Error())
			}
		}
	}
}

func TestMain(m *testing.M) {
	// logger.SetLogLevel("antsdb", "Debug")
	os.Exit(m.Run())
}

func TestNewClose(t *testing.T) {
	adb, _ := makeTestingHost(t)
	adb.Close()
}

type dbObj struct {
	Namespace string
	Id        string
	FileName  string
	CreatedAt int64
	UpdatedAt int64
}

func (t *dbObj) GetNamespace() string { return t.Namespace }

func (t *dbObj) GetID() string { return t.Id }

func (t *dbObj) Marshal() ([]byte, error) { return json.Marshal(t) }

func (t *dbObj) Unmarshal(val []byte) error { return json.Unmarshal(val, t) }

func (t *dbObj) SetCreated(unixTime int64) { t.CreatedAt = unixTime }

func (t *dbObj) SetUpdated(unixTime int64) { t.UpdatedAt = unixTime }

func (t *dbObj) GetCreated() int64 { return t.CreatedAt }

func (t *dbObj) GetUpdated() int64 { return t.UpdatedAt }

func factory(ns string) func() store.Item {
	return func() store.Item {
		return &dbObj{Namespace: ns}
	}
}

func TestSingleHostCRUD(t *testing.T) {
	adb, _ := makeTestingHost(t)
	defer adb.Close()

	d := &dbObj{
		Namespace: "antsObj",
		Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		FileName:  "MyTestFile.txt",
	}
	err := adb.Create(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed creating new object Err: ", err.Error())
	}

	d2 := &dbObj{
		Namespace: "antsObj",
		Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
	}
	err = adb.Read(context.TODO(), d2)
	if err != nil {
		t.Fatal("Failed reading object Err: ", err.Error())
	}

	if d2.FileName != d.FileName {
		t.Fatal("Object mismatch after read")
	}

	createdAt := d2.GetCreated()
	d.FileName = "MyUpdatedTestFile.txt"
	// Allow second for timestamps to be different
	<-time.After(time.Second)

	err = adb.Update(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed updating new object Err: ", err.Error())
	}

	err = adb.Read(context.TODO(), d2)
	if err != nil {
		t.Fatal("Failed reading object Err: ", err.Error())
	}

	if d2.FileName != d.FileName {
		t.Fatal("Object mismatch after read")
	}
	if d2.CreatedAt != createdAt || d2.UpdatedAt <= createdAt {
		t.Fatal("Timestamps incorrect", d2.CreatedAt, d2.UpdatedAt)
	}

	err = adb.Delete(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed deleting record", err.Error())
	}
	err = adb.Read(context.TODO(), d2)
	if err == nil {
		t.Fatal("Able to read object after deleting", d2)
	}
}

func TestMultiHostCRUD(t *testing.T) {
	adb1, h1 := makeTestingHost(t)
	defer adb1.Close()

	adb2, h2 := makeTestingHost(t)
	defer adb2.Close()

	adb3, h3 := makeTestingHost(t)
	defer adb3.Close()

	connectHosts(t, h1, h2, h3)

	d := &dbObj{
		Namespace: "antsObj",
		Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		FileName:  "MyTestFile.txt",
	}
	err := adb1.Create(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed creating new object Err: ", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb2, adb3} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err != nil {
			t.Fatal("Failed reading object Err: ", err.Error())
		}
		if d2.FileName != d.FileName {
			t.Fatal("Object mismatch after read")
		}
	}

	d.FileName = "MyUpdatedTestFile.txt"
	// Allow second for timestamps to be different
	<-time.After(time.Second)

	err = adb2.Update(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed updating new object Err: ", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb1, adb3} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err != nil {
			t.Fatal("Failed reading object Err: ", err.Error())
		}
		if d2.FileName != d.FileName {
			t.Fatal("Object mismatch after read")
		}
	}

	err = adb3.Delete(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed deleting record", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb1, adb2} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err == nil {
			t.Fatal("Able to reading object after delete")
		}
	}
}

func TestChannelValidator(t *testing.T) {
	adb1, h1 := makeTestingHost(t, WithChannel("ant1"))
	defer adb1.Close()

	adb2, h2 := makeTestingHost(t, WithChannel("ant2"))
	defer adb2.Close()

	adb3, h3 := makeTestingHost(t, WithPeerValidator(
		func(_ context.Context, p peer.ID) bool {
			if p == h1.ID() {
				return false
			}
			return true
		},
	))
	defer adb3.Close()

	connectHosts(t, h1, h2, h3)

	d := &dbObj{
		Namespace: "antsObj",
		Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		FileName:  "MyTestFile.txt",
	}
	err := adb1.Create(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed creating new object Err: ", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb2, adb3} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err == nil {
			t.Fatal("Able to read object Err: ", err.Error())
		}
	}

	d.FileName = "MyUpdatedTestFile.txt"
	// Allow second for timestamps to be different
	<-time.After(time.Second)

	err = adb1.Update(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed updating new object Err: ", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb2, adb3} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err == nil {
			t.Fatal("Able to read object Err: ", err.Error())
		}
	}

	err = adb1.Delete(context.TODO(), d)
	if err != nil {
		t.Fatal("Failed deleting record", err.Error())
	}
	// Allow update to propogate
	<-time.After(time.Second * 3)

	for _, db := range []*AntsDB{adb2, adb3} {
		d2 := &dbObj{
			Namespace: "antsObj",
			Id:        "04791e92-0b85-11ea-8d71-362b9e155667",
		}
		err = db.Read(context.TODO(), d2)
		if err == nil {
			t.Fatal("Able to reading object after delete")
		}
	}
}

func TestMultiHostList(t *testing.T) {
	adb1, h1 := makeTestingHost(t)
	defer adb1.Close()

	adb2, h2 := makeTestingHost(t)
	defer adb2.Close()

	connectHosts(t, h1, h2)
	// Create some dummies with antsObj namespace
	for i := 0; i < 5; i++ {
		d := dbObj{
			Namespace: "antsObj",
			Id:        fmt.Sprintf("%d", i),
		}
		err := adb1.Create(context.TODO(), &d)
		if err != nil {
			t.Fatalf(err.Error())
		}
		<-time.After(time.Second * 1)
	}
	//Create some dummies with Other namespace
	for i := 0; i < 5; i++ {
		d := dbObj{
			Namespace: "Other",
			Id:        fmt.Sprintf("%d", i),
		}
		err := adb1.Create(context.TODO(), &d)
		if err != nil {
			t.Fatalf(err.Error())
		}
		<-time.After(time.Second * 1)
	}
	opts := store.ListOpt{
		Page:  0,
		Limit: 6,
	}

	list, err := adb2.List(context.TODO(), factory("antsObj"), opts)
	if err != nil {
		t.Fatalf(err.Error())
	}
	count := 0
	for it := range list {
		if it.Err != nil {
			t.Fatal(it.Err)
		}
		if it.Val.GetNamespace() != "antsObj" {
			t.Fatalf("Namespace of the %vth element in list dosn't match", count)
		}
		count++
	}
	if count != 5 {
		t.Fatal("count mismatch during list", count)
	}
	// SortCreatedDesc
	opts.Sort = store.SortCreatedDesc
	list, err = adb2.List(context.TODO(), factory("antsObj"), opts)
	if err != nil {
		t.Fatalf(err.Error())
	}
	count = 0
	var oldTs int64 = 0
	for it := range list {
		if it.Err != nil {
			t.Fatal(it.Err)
		}
		if it.Val.GetNamespace() != "antsObj" {
			t.Fatalf("Namespace of the %vth element in list dosn't match", count)
		}
		if oldTs != 0 && it.Val.(*dbObj).CreatedAt > oldTs {
			t.Fatal("Order incorrect", oldTs, it.Val.(*dbObj).CreatedAt)
		}
		oldTs = it.Val.(*dbObj).CreatedAt
		count++
	}
	if count != 5 {
		t.Fatal("count mismatch during list", count)
	}
	// SortUpdatedAsc
	opts.Sort = store.SortUpdatedAsc
	list, err = adb2.List(context.TODO(), factory("antsObj"), opts)
	if err != nil {
		t.Fatalf(err.Error())
	}
	count = 0
	oldTs = 0
	for it := range list {
		if it.Err != nil {
			t.Fatal(it.Err)
		}
		if it.Val.GetNamespace() != "antsObj" {
			t.Fatalf("Namespace of the %vth element in list dosn't match", count)
		}
		if it.Val.(*dbObj).UpdatedAt < oldTs {
			t.Fatalf("Order incorrect")
		}
		oldTs = it.Val.(*dbObj).UpdatedAt
		count++
	}
	if count != 5 {
		t.Fatalf("count mismatch during list")
	}
}
