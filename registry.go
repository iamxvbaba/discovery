package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/etcd/clientv3"
)

// 服务信息
type ServiceInfo struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Version string `json:"version"`
	Delete  bool   `json:"delete"` // true 表示移除服务
}

type Service struct {
	ServiceInfo *ServiceInfo
	stop        chan struct{}
	leaseId     clientv3.LeaseID
	client      *clientv3.Client
}

// NewService 创建一个注册服务
func NewService(info *ServiceInfo, endpoints []string) (service *Service, err error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Second * 10,
	})

	if err != nil {
		return nil, err
	}
	Log.SetPrefix("[registry]")
	service = &Service{
		ServiceInfo: info,
		client:      client,
	}
	return
}

// Start 注册服务启动
func (service *Service) Start() {
	retry := 0
re:
	recvs := 0
	for { // 网络抖动导致服务不可用的重试
		if retry > 0 {
			Log.Printf("%s keepAlive retry %d times error \n", service.getKey(), retry)
		}
		ch, err := service.keepAlive()
		if err != nil {
			Log.Printf("%s service Start routine error:%v \n", service.getKey(), err)
			retry++
			time.Sleep(time.Second)
			goto re
		}
		for {
			select {
			case <-service.stop:
				Log.Printf("%s service stop \n", service.getKey())
				return
			case <-service.client.Ctx().Done():
				Log.Printf("%s service done \n", service.getKey())
				return
			case resp, ok := <-ch:
				// 监听租约
				if !ok {
					Log.Println("keep alive channel closed")
					if err = service.revoke(); err != nil {
						Log.Printf("%s service revoke error:%v \n", service.getKey(), err)
					}
					retry++
					time.Sleep(time.Second)
					goto re
				}
				recvs++
				if recvs%5 == 0 {
					Log.Printf("Recv reply from service: %s, ttl:%d \n", service.getKey(), resp.TTL)
				}
			}
		}
	}
}

func (service *Service) Stop() {
	service.stop <- struct{}{}
}

func (service *Service) keepAlive() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	var (
		info   = &service.ServiceInfo
		key    = service.getKey()
		val, _ = json.Marshal(info)
	)
	ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
	defer fn()
	// 创建一个租约
	resp, err := service.client.Grant(ctx, 6)
	if err != nil {
		return nil, err
	}
	_, err = service.client.Put(ctx, key, string(val), clientv3.WithLease(resp.ID))
	if err != nil {
		return nil, err
	}
	service.leaseId = resp.ID
	return service.client.KeepAlive(context.Background(), resp.ID)
}

func (service *Service) revoke() error {
	ctx, fn := context.WithTimeout(context.Background(), 3*time.Second)
	defer fn()
	_, err := service.client.Revoke(ctx, service.leaseId)
	return err
}

func (service *Service) getKey() string {
	return fmt.Sprintf("%s%s",
		fmt.Sprintf("/%s/%s/", service.ServiceInfo.Name, service.ServiceInfo.Version), service.ServiceInfo.Addr)
}
