package pubsub

import (
	"GoRedis/datastruct/list"
	"GoRedis/interface/resp"
	"GoRedis/lib/utils"
	"GoRedis/resp/reply"
	"strconv"
)

var (
	_subscribe         = "subscribe"
	_unsubscribe       = "unsubscribe"
	messageBytes       = []byte("message")
	unSubscribeNothing = []byte("*3\r\n$11\r\nunsubscribe\r\n$-1\n:0\r\n")
)

func makeMsg(t string, channel string, code int64) []byte {
	return []byte("*3\r\n$" + strconv.FormatInt(int64(len(t)), 10) + reply.CRLF + t + reply.CRLF +
		"$" + strconv.FormatInt(int64(len(channel)), 10) + reply.CRLF + channel + reply.CRLF +
		":" + strconv.FormatInt(code, 10) + reply.CRLF)
}

/*
 * invoker should lock channel
 * return: is new subscribed
 */
// 订阅操作
func subscribe0(hub *Hub, channel string, client resp.Connection) bool {
	// 将客户端订阅到该频道
	client.Subscribe(channel)

	// 获取该频道的订阅者链表
	raw, ok := hub.subs.Get(channel)
	var subscribers *list.LinkedList
	// 如果该频道存在
	if ok {
		subscribers, _ = raw.(*list.LinkedList)
		// 频道不存在
	} else {
		// 构造订阅者链表
		subscribers = list.Make()
		// 将频道存入map
		hub.subs.Put(channel, subscribers)
	}
	// 判断订阅者链表中是否已经有这个订阅者
	if subscribers.Contains(func(a interface{}) bool {
		return a == client
	}) {
		return false
	}
	// 如果订阅者链表中没有该订阅者，就将该订阅者存入订阅者链表
	subscribers.Add(client)
	return true
}

/*
 * invoker should lock channel
 * return: is actually un-subscribe
 */
// 退订操作
func unsubscribe0(hub *Hub, channel string, client resp.Connection) bool {
	client.UnSubscribe(channel)

	raw, ok := hub.subs.Get(channel)
	if ok {
		subscribers, _ := raw.(*list.LinkedList)
		subscribers.RemoveAllByVal(func(a interface{}) bool {
			return utils.Equals(a, client)
		})

		if subscribers.Len() == 0 {
			// clean
			hub.subs.Remove(channel)
		}
		return true
	}
	return false
}

// Subscribe 将客户端订阅到给定的频道上
func Subscribe(hub *Hub, c resp.Connection, args [][]byte) resp.Reply {
	channels := make([]string, len(args))
	for i, b := range args {
		channels[i] = string(b)
	}

	for _, channel := range channels {
		if subscribe0(hub, channel, c) {
			// 向客户端返回订阅的频道数量
			_ = c.Write(makeMsg(_subscribe, channel, int64(c.SubsCount())))
		}
	}
	return &reply.NoReply{}
}

// UnsubscribeAll 退订所有频道
func UnsubscribeAll(hub *Hub, c resp.Connection) {
	channels := c.GetChannels()

	for _, channel := range channels {
		unsubscribe0(hub, channel, c)
	}

}

// UnSubscribe 客户端将给定的频道都退订
func UnSubscribe(db *Hub, c resp.Connection, args [][]byte) resp.Reply {
	var channels []string
	if len(args) > 0 {
		channels = make([]string, len(args))
		for i, b := range args {
			channels[i] = string(b)
		}
	} else {
		channels = c.GetChannels()
	}

	if len(channels) == 0 {
		_ = c.Write(unSubscribeNothing)
		return &reply.NoReply{}
	}

	for _, channel := range channels {
		if unsubscribe0(db, channel, c) {
			// 返回退订的频道，以及当前订阅的频道数量
			_ = c.Write(makeMsg(_unsubscribe, channel, int64(c.SubsCount())))
		}
	}
	return &reply.NoReply{}
}

// Publish 向订阅频道的所有订阅者发送消息
func Publish(hub *Hub, args [][]byte) resp.Reply {
	if len(args) != 2 {
		return &reply.ArgNumErrReply{Cmd: "publish"}
	}
	channel := string(args[0])
	message := args[1]

	// 从map中取出频道以及订阅该频道的订阅者链表
	raw, ok := hub.subs.Get(channel)
	if !ok {
		return reply.MakeIntReply(0)
	}
	subscribers, _ := raw.(*list.LinkedList)
	subscribers.ForEach(func(i int, c interface{}) bool {
		client, _ := c.(resp.Connection)
		// 发送的消息分为三个部分：
		// 1. "message" 固定字段
		// 2. 频道名称
		// 3. 消息本体
		replyArgs := make([][]byte, 3)
		replyArgs[0] = messageBytes
		replyArgs[1] = []byte(channel)
		replyArgs[2] = message
		// 向客户端发送消息
		_ = client.Write(reply.MakeMultiBulkReply(replyArgs).ToBytes())
		return true
	})
	return reply.MakeIntReply(int64(subscribers.Len()))
}
