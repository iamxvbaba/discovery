package discovery

import (
	"context"
	"encoding/json"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/resolver"
	"time"
)

type Watcher struct {
	cli    *clientv3.Client
	prefix string
	notify chan ServiceInfo
	addrId map[string]int64
}

func NewWatcher(etcd []string, service string) *Watcher {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcd,
		DialTimeout: time.Second * 5,
	})
	if err != nil {
		Log.Printf("watcher connect to etcd error:%v \n", err)
		return nil
	}
	w := &Watcher{
		cli:    cli,
		prefix: service,
		notify: make(chan ServiceInfo, 5),
		addrId: make(map[string]int64, 5),
	}
	go w.start()
	return w
}

func (w *Watcher) start() {
	addrDict := make(map[string]resolver.Address)
	update := func() {
		addrList := make([]resolver.Address, 0, len(addrDict))
		for _, v := range addrDict {
			addrList = append(addrList, v)
		}
	}

	// 首次获取所有的服务
	resp, err := w.cli.Get(context.Background(), w.prefix, clientv3.WithPrefix())
	if err != nil {
		panic(err)
	}
	for i, kv := range resp.Kvs {
		var info ServiceInfo
		err = json.Unmarshal(kv.Value, &info)
		if err != nil {
			panic(err)
		}
		// 通知外部
		w.changeNotify(info)
		addrDict[string(resp.Kvs[i].Key)] = resolver.Address{Addr: info.Addr, ServerName: info.Name}
	}
	update()
	Log.Printf("watch prefix:%s\n", w.prefix)
	rch := w.cli.Watch(context.Background(), w.prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for n := range rch {
		for _, ev := range n.Events {
			key := string(ev.Kv.Key)
			switch ev.Type {
			case mvccpb.PUT:
				var info ServiceInfo
				if err := json.Unmarshal(ev.Kv.Value, &info); err != nil {
					Log.Printf("ev.Kv.Value json.Unmarshal error:%v \n", err)
				}
				addrDict[key] = resolver.Address{Addr: info.Addr, ServerName: info.Name}
				Log.Printf("[增加服务] key:%v server_name:%v addr:%v \n", key, info.Name, info.Addr)
				// 通知外部
				info.Delete = false
				w.changeNotify(info)
			case mvccpb.DELETE:
				Log.Printf("[移除服务] key:%v server_name:%v addr:%v \n", key,
					addrDict[key].ServerName, addrDict[key].Addr)
				// 通知外部
				w.changeNotify(ServiceInfo{
					ID:     w.addrId[addrDict[key].Addr],
					Addr:   addrDict[key].Addr,
					Delete: true,
				})
				delete(addrDict, string(ev.PrevKv.Key))
			}
		}
		update()
	}
}
func (w *Watcher) changeNotify(srv ServiceInfo) {
	Log.Printf("srv change:%v", srv)
	w.addrId[srv.Addr] = srv.ID
	select {
	case w.notify <- srv:
	default:
		Log.Printf("srv change no listen:%v", srv)
	}
}

func (w *Watcher) Watch() chan ServiceInfo {
	return w.notify
}
