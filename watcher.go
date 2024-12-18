package discovery

import (
	"context"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	etcd "go.etcd.io/etcd/client/v3"
)

type WatchFn func(bool, []byte)

type Watcher struct {
	cli      *etcd.Client
	prefixFn map[string][]WatchFn // key,callback
}

func NewWatcher(ctx context.Context, eCfg EtcdConfig, prefixFn map[string][]WatchFn) *Watcher {
	cli, err := etcd.New(etcd.Config{
		DialTimeout: time.Second * 3,
		Endpoints:   eCfg.Endpoints,
		Username:    eCfg.Username,
		Password:    eCfg.Password,
	})
	if err != nil {
		panic(err)
	}

	w := &Watcher{
		cli:      cli,
		prefixFn: prefixFn,
	}

	// 防止连完etcd程序已经停止了
	select {
	case <-ctx.Done():
		panic("stopped by " + ctx.Err().Error())
	default:
	}
	w.run(ctx)

	return w
}

func (w *Watcher) run(ctx context.Context) {
	for pre, fn := range w.prefixFn {
		go w.listen(ctx, pre, fn)
	}
}

func (w *Watcher) listen(ctx context.Context, prefix string, callbacks []WatchFn) {
	values := make(map[string][]byte)
	timeoutCtx, fn := context.WithTimeout(context.Background(), time.Second*3)
	defer fn()

	resp, err := w.cli.Get(timeoutCtx, prefix, etcd.WithPrefix())
	if err != nil {
		panic(err)
	}
	if len(resp.Kvs) == 0 {
		Log.Printf("当前ETCD没有值 %s\n", prefix)
	}

	for _, kv := range resp.Kvs {
		values[string(kv.Key)] = kv.Value
		for _, callback := range callbacks {
			callback(true, kv.Value)
		}
	}

	Log.Printf("watch prefix:%s\n", prefix)
	rch := w.cli.Watch(context.Background(), prefix, etcd.WithPrefix(), etcd.WithPrevKV())
	for {
		select {
		case <-ctx.Done():
			Log.Printf("watcher exit\n")
			return
		case n := <-rch:
			for _, ev := range n.Events {
				key := string(ev.Kv.Key)
				switch ev.Type {
				case mvccpb.PUT:
					Log.Printf("[ECTD增加] key:%v value:%s\n",
						key, string(ev.Kv.Value))
					values[string(ev.Kv.Key)] = ev.Kv.Value
					for _, callback := range callbacks {
						callback(true, ev.Kv.Value)
					}
				case mvccpb.DELETE:
					oldVal := values[key]
					Log.Printf("[ECTD移除] key:%v value:%s\n", key, string(oldVal))
					delete(values, string(ev.Kv.Key))
					for _, callback := range callbacks {
						callback(false, oldVal)
					}
				}
			}
		}
	}
}
