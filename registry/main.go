package main

import (
	"fmt"
	"github.com/iamxvbaba/discovery"
)

func main() {
	var (
		etcdAddr = []string{"127.0.0.1:2379"}
	)
	registry(etcdAddr)
	watch(etcdAddr)
	fmt.Println("启动")
	select {}
}

func registry(etcd []string) {
	var (
		regSrv *discovery.Service
		err    error
	)
	// etcd服务注册
	if regSrv, err = discovery.NewService(&discovery.ServiceInfo{
		ID:      1,
		Name:    "test",
		Addr:    "test_version",
		Version: "test_version",
	}, etcd); err != nil {
		panic(err)
	}
	go regSrv.Start()
}

func watch(etcd []string) {
	watcher := discovery.NewWatcher(etcd, fmt.Sprintf("/%s/%s/", "test", "test_version"))
	if watcher == nil {
		return
	}
	go func() {
		for srv := range watcher.Watch() {
			fmt.Printf("服务发生变化:%v\n", srv)
		}
	}()

}
