package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	etcd "go.etcd.io/etcd/client/v3"
)

func TestRegister(t *testing.T) {
	etcd := EtcdConfig{
		Endpoints: []string{"127.0.0.1:2379"},
		Username:  "root",
		Password:  "qwer1234",
	}
	Register(context.Background(), etcd, &ServiceInfo{
		ID:       1,
		Name:     "hall",
		Internal: "127.0.0.1",
		Version:  "1.0.0",
	})
}

func TestEtcdQuery(t *testing.T) {
	eCfg := EtcdConfig{
		Endpoints: []string{"120.26.90.21:2379"},
		Username:  "root",
		Password:  "qwer1234",
	}

	client, err := etcd.New(etcd.Config{
		DialTimeout: time.Second * 3,
		Endpoints:   eCfg.Endpoints,
		Username:    eCfg.Username,
		Password:    eCfg.Password,
	})
	if err != nil {
		panic(err)
	}

	resp, err := client.Get(context.Background(), "/", etcd.WithPrefix())
	if err != nil {
		panic(err)
	}

	for _, kv := range resp.Kvs {
		fmt.Printf("key:%s, value:%s\n", kv.Key, kv.Value)
	}

}
