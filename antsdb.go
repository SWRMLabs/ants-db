package antsdb

import (
	"context"
	"time"

	ds "github.com/ipfs/go-datastore"
	crdt "github.com/ipfs/go-ds-crdt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	multihash "github.com/multiformats/go-multihash"
	store "github.com/plexsysio/gkvstore"
	dsStore "github.com/plexsysio/gkvstore-ipfsds"
)

var (
	defaultRootNs = "/ant"
	defaultTopic  = "antWorker"
	log           = logging.Logger("antsdb")
)

type Option func(a *AntsDB)

func WithChannel(topic string) Option {
	return func(a *AntsDB) {
		a.topicName = topic
	}
}

func WithPeerValidator(validator func(context.Context, peer.ID) bool) Option {
	return func(a *AntsDB) {
		a.validator = validator
	}
}

func WithNamespace(ns string) Option {
	return func(a *AntsDB) {
		a.namespace = ds.NewKey(ns)
	}
}

func WithRebroadcastDuration(d time.Duration) Option {
	return func(a *AntsDB) {
		a.rebcastInterval = d
	}
}

func WithOnCloseHook(hook func()) Option {
	return func(a *AntsDB) {
		a.addOnClose(hook)
	}
}

type Subscriber interface {
	Put(string)
	Delete(string)
}

func WithSubscriber(s Subscriber) Option {
	return func(a *AntsDB) {
		a.subscriber = s
	}
}

func defaultOpts(a *AntsDB) {
	if len(a.namespace.String()) == 0 {
		a.namespace = ds.NewKey(defaultRootNs)
	}
	if len(a.topicName) == 0 {
		a.topicName = defaultTopic
	}
	if a.rebcastInterval == 0 {
		a.rebcastInterval = time.Second
	}
}

type AntsDB struct {
	ctx             context.Context
	cancel          context.CancelFunc
	syncer          crdt.SessionDAGSyncer
	pubsub          *pubsub.PubSub
	storage         ds.Batching
	namespace       ds.Key
	subscriber      Subscriber
	topicName       string
	rebcastInterval time.Duration
	validator       func(context.Context, peer.ID) bool
	closers         []func()

	store.Store
}

func New(
	syncer crdt.SessionDAGSyncer,
	pubsub *pubsub.PubSub,
	store ds.Batching,
	opts ...Option,
) (*AntsDB, error) {

	ctx, cancel := context.WithCancel(context.Background())

	adb := &AntsDB{
		ctx:     ctx,
		cancel:  cancel,
		syncer:  syncer,
		pubsub:  pubsub,
		storage: store,
	}
	for _, opt := range opts {
		opt(adb)
	}
	defaultOpts(adb)
	return adb, adb.setup()
}

func (a *AntsDB) setup() error {
	topicHash, err := multihash.Sum([]byte(a.topicName), multihash.MD5, -1)
	if err == nil {
		log.Infof("Updating topic name with hash %s", topicHash)
		a.topicName = topicHash.B58String()
	}
	if a.validator != nil {
		err = a.pubsub.RegisterTopicValidator(
			a.topicName,
			func(ctx context.Context, p peer.ID, msg *pubsub.Message) bool {
				return a.validator(ctx, p)
			},
		)
		if err != nil {
			log.Errorf("Failed registering pubsub topic Err:%s", err.Error())
			return err
		}
	}
	broadcaster, err := crdt.NewPubSubBroadcaster(
		a.ctx,
		a.pubsub,
		a.topicName,
	)
	if err != nil {
		log.Errorf("Failed creating broadcaster Err:%s", err.Error())
		return err
	}
	opts := crdt.DefaultOptions()
	opts.RebroadcastInterval = a.rebcastInterval
	opts.DAGSyncerTimeout = 2 * time.Minute
	opts.Logger = log
	if a.subscriber != nil {
		opts.PutHook = func(k ds.Key, v []byte) {
			log.Infof("AntsDB PUT %s", k)
			a.subscriber.Put(k.String())
		}
		opts.DeleteHook = func(k ds.Key) {
			log.Infof("AntsDB DELETE %s", k)
			a.subscriber.Delete(k.String())
		}
	}
	crdt, err := crdt.New(
		a.storage,
		a.namespace,
		a.syncer,
		broadcaster,
		opts,
	)
	if err != nil {
		log.Errorf("Failed creating crdt datastore Err:%s", err.Error())
		return err
	}
	a.Store = dsStore.New(crdt)
	a.addOnClose(func() {
		log.Info("Stopping AntsDB")
		a.cancel()
		log.Info("Closing CRDT datastore")
		crdt.Close()
	})
	return nil
}

func (a *AntsDB) addOnClose(hook func()) {
	if a.closers == nil {
		a.closers = []func(){hook}
		return
	}
	a.closers = append(a.closers, hook)
}

func (a *AntsDB) Close() error {
	log.Info("Closing AntsDB")
	for _, stop := range a.closers {
		stop()
	}
	return nil
}
