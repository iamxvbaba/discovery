package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints" toml:"endpoints"`
	Username  string   `yaml:"username" toml:"username"`
	Password  string   `yaml:"password" toml:"password"`
}

type RegistryInfo struct {
	Name         string `yaml:"name" toml:"name"`
	Version      string `yaml:"version" toml:"version"`
	Port         int    `yaml:"port" toml:"port"`
	ExternalAddr string `yaml:"external_addr" toml:"external_addr"` // 对外暴露地址
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	ID       int64          `json:"id" toml:"id"`
	Type     int64          `json:"type" toml:"type"`
	Name     string         `json:"name" toml:"name"`
	UUID     string         `json:"uuid" toml:"uuid"`
	Version  string         `json:"version" toml:"version"`
	Internal string         `json:"internal" toml:"internal"`
	External string         `json:"external" toml:"external"`
	Ext      map[string]any `json:"ext" toml:"ext"` // 新增扩展信息
}

type Service struct {
	ServiceInfo *ServiceInfo
	leaseId     clientv3.LeaseID
	client      *clientv3.Client
	key         string
}

// Register 注册服务
func Register(ctx context.Context, eCfg EtcdConfig, info *ServiceInfo) {
	client, err := clientv3.New(clientv3.Config{
		DialTimeout: time.Second * 3,
		Endpoints:   eCfg.Endpoints,
		Username:    eCfg.Username,
		Password:    eCfg.Password,
	})
	if err != nil {
		panic(err)
	}
	if len(info.UUID) == 0 {
		info.UUID = uuid.NewString()
	}
	s := &Service{
		ServiceInfo: info,
		client:      client,
	}
	// 防止连完etcd程序已经停止了
	select {
	case <-ctx.Done():
		Log.Printf("discovery stopped by %v\n", ctx.Err())
		return
	default:
	}
	go s.run(ctx)
}

func (s *Service) run(ctx context.Context) {
	preKey := BuildPrefixKey(s.ServiceInfo.Name, s.ServiceInfo.Version)
	s.key = fmt.Sprintf("%s/%s", preKey, s.ServiceInfo.Internal)

	retry := 0
re:
	recvs := 0
	for { // 网络抖动导致服务不可用的重试
		if retry > 0 {
			Log.Printf("%s keepAlive retry %d times error\n", s.key, retry)
		}
		ch, err := s.keepAlive()
		if err != nil {
			Log.Printf("%s s Start routine error:%v\n", s.key, err)
			retry++
			time.Sleep(time.Second)
			goto re
		}
		for {
			select {
			case <-ctx.Done():
				Log.Printf("%s done\n", s.key)
				_ = s.revoke()
				return
			case <-s.client.Ctx().Done():
				_ = s.revoke()
				Log.Printf("%s done\n", s.key)
				return
			case resp, ok := <-ch:
				// 监听租约
				if !ok {
					Log.Printf("keep alive channel closed\n")
					if err = s.revoke(); err != nil {
						Log.Printf("%s s revoke error:%v\n", s.key, err)
					}
					retry++
					time.Sleep(time.Second)
					goto re
				}
				recvs++
				if recvs%60 == 0 {
					Log.Printf("Recv reply from s: %s, ttl:%d\n", s.key, resp.TTL)
				}
			}
		}
	}
}

func (s *Service) keepAlive() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	val, _ := json.Marshal(&s.ServiceInfo)
	ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
	defer fn()
	// 创建一个租约
	resp, err := s.client.Grant(ctx, 6)
	if err != nil {
		return nil, err
	}
	_, err = s.client.Put(ctx, s.key, string(val), clientv3.WithLease(resp.ID))
	if err != nil {
		return nil, err
	}
	s.leaseId = resp.ID
	return s.client.KeepAlive(context.Background(), resp.ID)
}

func (s *Service) revoke() error {
	ctx, fn := context.WithTimeout(context.Background(), 3*time.Second)
	defer fn()
	_, err := s.client.Revoke(ctx, s.leaseId)
	return err
}

func BuildPrefixKey(name, version string) string {
	return fmt.Sprintf("/%s/%s", name, version)
}
