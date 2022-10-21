package pubsub

import "GoRedis/datastruct/dict"

// Hub 储存所有的订阅关系
// 该map是map[string]*List结构，List中存的是客户端结构体，储存的是频道和频道的订阅者链表
type Hub struct {
	// channel -> list(*Client)
	subs dict.Dict
}

// MakeHub creates new hub
func MakeHub() *Hub {
	return &Hub{
		subs: dict.MakeSyncDict(),
	}
}
