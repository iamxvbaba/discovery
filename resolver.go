package discovery

import (
	"context"
	"encoding/json"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
	"sync/atomic"
	"time"
)

type Resolver struct {
	ctx        context.Context
	prefix     string
	client     *clientv3.Client
	cc         resolver.ClientConn
	serviceMap map[string]ServiceInfo
	running    atomic.Bool
}

func NewResolver(ctx context.Context, eCfg EtcdConfig, prefix string) *Resolver {
	cli, err := clientv3.New(clientv3.Config{
		DialTimeout: time.Second * 3,
		Endpoints:   eCfg.Endpoints,
		Username:    eCfg.Username,
		Password:    eCfg.Password,
	})
	if err != nil {
		panic(err)
	}
	r := &Resolver{
		ctx:    ctx,
		prefix: prefix,
		client: cli,
	}
	// 防止连完etcd程序已经停止了
	select {
	case <-ctx.Done():
		panic("stopped by " + ctx.Err().Error())
	default:
	}
	return r
}

func (r *Resolver) Build(_ resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	Log.Printf("Resolver Build\n")
	r.cc = cc
	if r.running.Load() {
		Log.Printf("Resolver already running\n")
		r.update()
		return r, nil
	}
	go r.run()
	return r, nil
}

func (r *Resolver) Scheme() string {
	return "etcd"
}

func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *Resolver) Close() {}

func (r *Resolver) update() {
	Log.Printf("Resolver update prefix=%s services_count=%d\n", r.prefix, len(r.serviceMap))
	if len(r.serviceMap) == 0 {
		return
	}
	addresses := make([]resolver.Address, 0, len(r.serviceMap))
	for _, srv := range r.serviceMap {
		Log.Printf("Resolver update prefix=%s name=%s addr=%s\n", r.prefix, srv.Name, srv.Internal)
		addresses = append(addresses, resolver.Address{
			Addr:       srv.Internal,
			ServerName: srv.Name,
		})
	}
	err := r.cc.UpdateState(resolver.State{Addresses: addresses})
	if err != nil {
		Log.Printf("Resolver UpdateState err %v\n", err.Error())
	}
}

func (r *Resolver) run() {
	r.running.Store(true)
	r.serviceMap = make(map[string]ServiceInfo)
	// 首次获取所有的服务,超时3s
	timeoutCtx, fn := context.WithTimeout(context.Background(), time.Second*3)
	defer fn()
	resp, err := r.client.Get(timeoutCtx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		panic(err)
	}
	if len(resp.Kvs) == 0 {
		Log.Printf("当前没有服务 prefix:%s\n", r.prefix)
	}
	for _, kv := range resp.Kvs {
		var info ServiceInfo
		err = json.Unmarshal(kv.Value, &info)
		if err != nil {
			panic(err)
		}
		r.serviceMap[string(kv.Key)] = info
	}
	r.update()
	//log.Debugf("resolver prefix:%s", r.prefix)
	rch := r.client.Watch(context.Background(), r.prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for {
		select {
		case <-r.ctx.Done():
			Log.Printf("Resolver exit\n")
			return
		case n := <-rch:
			for _, ev := range n.Events {
				key := string(ev.Kv.Key)
				switch ev.Type {
				case mvccpb.PUT:
					var info ServiceInfo
					if err := json.Unmarshal(ev.Kv.Value, &info); err != nil {
						Log.Printf("ev.Kv.Value json.Unmarshal error:%v\n", err)
						continue
					}
					Log.Printf("[增加服务] key:%v server:%v id:%d internal:%v external:%s\n",
						key, info.Name, info.ID, info.Internal, info.External)
					r.serviceMap[key] = info
					r.update()
				case mvccpb.DELETE:
					srvInfo := r.serviceMap[key]
					Log.Printf("[移除服务] key:%v server:%v id:%d internal:%s external:%s\n", key,
						srvInfo.Name, srvInfo.ID, srvInfo.Internal, srvInfo.External)
					delete(r.serviceMap, string(ev.PrevKv.Key))
					r.update()
				}
			}
		}
	}
}
