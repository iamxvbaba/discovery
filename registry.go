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

	service = &Service{
		ServiceInfo: info,
		client:      client,
	}
	return
}

// Start 注册服务启动
func (service *Service) Start() {
	ch, err := service.keepAlive()
	if err != nil {
		fmt.Errorf("%s service Start routine error:%v \n", service.getKey(), err)
		return
	}
	for {
		select {
		case <-service.stop:
			fmt.Errorf("%s service stop \n", service.getKey())
			return
		case <-service.client.Ctx().Done():
			fmt.Errorf("%s service done \n", service.getKey())
			return
		case _, ok := <-ch:
			// 监听租约
			if !ok {
				fmt.Println("keep alive channel closed")
				if err = service.revoke(); err != nil {
					fmt.Errorf("%s service revoke error:%v \n", service.getKey(), err)
				}
				return
			}
			// log.Infof("Recv reply from service: %s, ttl:%d", service.getKey(), resp.TTL)
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
	// 创建一个租约
	resp, err := service.client.Grant(context.TODO(), 5)
	if err != nil {
		return nil, err
	}
	_, err = service.client.Put(context.TODO(), key, string(val), clientv3.WithLease(resp.ID))
	if err != nil {
		return nil, err
	}
	service.leaseId = resp.ID
	return service.client.KeepAlive(context.TODO(), resp.ID)
}

func (service *Service) revoke() error {
	_, err := service.client.Revoke(context.TODO(), service.leaseId)
	return err
}

func (service *Service) getKey() string {
	return fmt.Sprintf("%s%s",
		fmt.Sprintf("/%s/%s/", service.ServiceInfo.Name, service.ServiceInfo.Version), service.ServiceInfo.Addr)
}
