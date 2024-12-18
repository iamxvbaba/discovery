package discovery

import (
	"sync"
	"sync/atomic"
)

// ServerManager 用于读写极其极端的情况下(10000:1),读基本无锁
type ServerManager struct {
	servers atomic.Value // 存储 map[uint32]string id -> uuid , 对外查询
	mutex   sync.Mutex   // 仅用于写操作
}

var defMgr = NewServerManager()

func NewServerManager() *ServerManager {
	mgr := &ServerManager{}
	mgr.servers.Store(make(map[uint32]string))
	return mgr
}

func AddServer(info ServiceInfo) {
	Log.Printf("NewServer serverID=%d type=%d uuid=%s internal=%s\n",
		info.ID, info.Type, info.UUID, info.Internal)
	defMgr.AddServer(uint32(info.ID), info.UUID)
}

func (mgr *ServerManager) AddServer(srvID uint32, uuid string) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	oldMap := mgr.servers.Load().(map[uint32]string)
	newMap := make(map[uint32]string, len(oldMap)+1)
	for k, v := range oldMap {
		newMap[k] = v
	}
	newMap[srvID] = uuid
	mgr.servers.Store(newMap)
}

func RemoveServer(info ServiceInfo) {
	Log.Printf("RemoveServer serverID=%d type=%d uuid=%s internal=%s\n",
		info.ID, info.Type, info.UUID, info.Internal)
	defMgr.RemoveServer(uint32(info.ID), info.UUID)
}

func (mgr *ServerManager) RemoveServer(srvID uint32, uuid string) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	oldMap := mgr.servers.Load().(map[uint32]string)
	if _, exists := oldMap[srvID]; !exists {
		return // 如果服务器不存在，直接返回
	}
	newMap := make(map[uint32]string, len(oldMap)-1)
	for k, v := range oldMap {
		if k == srvID && v == uuid {
			continue
		}
		newMap[k] = v
	}
	mgr.servers.Store(newMap)
}

func ServerActive(srvID uint32) bool {
	return defMgr.ServerActive(srvID)
}

func (mgr *ServerManager) ServerActive(srvID uint32) bool {
	servers := mgr.servers.Load().(map[uint32]string)
	_, exists := servers[srvID]
	return exists
}

func Servers() map[uint32]string {
	return defMgr.servers.Load().(map[uint32]string)
}
