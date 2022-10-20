package pubsub

import "GoRedis/datastruct/dict"

// Hub stores all subscribe relations
// 储存所有的订阅关系
type Hub struct {
	// channel -> list(*Client)
	subs dict.Dict
	//// lock channel
	//subsLocker *lock.Locks
}

// MakeHub creates new hub
func MakeHub() *Hub {
	return &Hub{
		subs: dict.MakeSyncDict(),
	}
}
