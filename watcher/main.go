package main

import (
	"fmt"
	"github.com/iamxvbaba/discovery"
)

func main() {
	var (
		etcdAddr = []string{"127.0.0.1:2379"}
	)
	watch(etcdAddr)
	fmt.Println("启动")
	select {

	}
}
func watch(etcd []string) {
	watcher := discovery.NewWatcher(etcd, fmt.Sprintf("/%s/%s/", "test", "test_version"))
	if watcher == nil {
		fmt.Println("watcher is nil")
		return
	}
	go func() {
		for srv := range watcher.Watch() {
			fmt.Printf("服务发生变化:%v\n", srv)
		}
	}()

}