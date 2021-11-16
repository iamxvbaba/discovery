package discovery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/resolver"
)

const schema = "etcd"

// resolver is the implementaion of grpc.resolve.Builder
// Resolver 实现grpc的grpc.resolve.Builder接口的Build与Scheme方法
type Resolver struct {
	endpoints []string
	service   string
	cli       *clientv3.Client
	cc        resolver.ClientConn
	notify    chan ServiceInfo
	addrId    map[string]int64
}

// NewResolver return resolver builder
// endpoints example: http://127.0.0.1:2379 http://127.0.0.1:12379 http://127.0.0.1:22379"
// service is service name
// notify is service changed notify
func NewResolver(endpoints []string, service string, notify chan ServiceInfo) resolver.Builder {
	return &Resolver{endpoints: endpoints, service: service, notify: notify, addrId: make(map[string]int64, 5)}
}

// Scheme return etcd schema
func (r *Resolver) Scheme() string {
	// 最好用这种，因为grpc resolver.Register(r)在注册时，会取scheme，如果一个系统有多个grpc发现，就会覆盖之前注册的
	return schema + "_" + r.service
}

// ResolveNow
func (r *Resolver) ResolveNow(rn resolver.ResolveNowOptions) {
}

// Close
func (r *Resolver) Close() {
}
func (r *Resolver) changeNotify(srv ServiceInfo) {
	r.addrId[srv.Addr] = srv.ID
	if r.notify != nil {
		r.notify <- srv
	}
}

// Build to resolver.Resolver
// 实现grpc.resolve.Builder接口的方法
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	var err error

	r.cli, err = clientv3.New(clientv3.Config{
		Endpoints: r.endpoints,
	})
	if err != nil {
		return nil, fmt.Errorf("grpclb: create clientv3 client failed: %v", err)
	}

	r.cc = cc

	go r.watch(fmt.Sprintf(r.service))

	return r, nil
}
func (r *Resolver) watch(prefix string) {
	addrDict := make(map[string]resolver.Address)
	update := func() {
		addrList := make([]resolver.Address, 0, len(addrDict))
		for _, v := range addrDict {
			addrList = append(addrList, v)
		}
		r.cc.UpdateState(resolver.State{
			Addresses:     addrList,
			ServiceConfig: nil,
			Attributes:    nil,
		})
	}

	// 首次获取所有的服务
	resp, err := r.cli.Get(context.Background(), prefix, clientv3.WithPrefix())
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
		r.changeNotify(info)
		addrDict[string(resp.Kvs[i].Key)] = resolver.Address{Addr: info.Addr, ServerName: info.Name}
	}
	update()

	rch := r.cli.Watch(context.Background(), prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
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
				r.changeNotify(info)
			case mvccpb.DELETE:
				Log.Printf("[移除服务] key:%v server_name:%v addr:%v \n", key,
					addrDict[key].ServerName, addrDict[key].Addr)
				// 通知外部
				r.changeNotify(ServiceInfo{
					ID:     r.addrId[addrDict[key].Addr],
					Addr:   addrDict[key].Addr,
					Delete: true,
				})
				delete(addrDict, string(ev.PrevKv.Key))
			}
		}
		update()
	}
}
