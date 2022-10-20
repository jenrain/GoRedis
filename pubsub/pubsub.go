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
func subscribe0(hub *Hub, channel string, client resp.Connection) bool {
	client.Subscribe(channel)

	// add into hub.subs
	raw, ok := hub.subs.Get(channel)
	var subscribers *list.LinkedList
	if ok {
		subscribers, _ = raw.(*list.LinkedList)
	} else {
		subscribers = list.Make()
		hub.subs.Put(channel, subscribers)
	}
	if subscribers.Contains(func(a interface{}) bool {
		return a == client
	}) {
		return false
	}
	subscribers.Add(client)
	return true
}

/*
 * invoker should lock channel
 * return: is actually un-subscribe
 */
func unsubscribe0(hub *Hub, channel string, client resp.Connection) bool {
	client.UnSubscribe(channel)

	// remove from hub.subs
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

// Subscribe puts the given connection into the given channel
func Subscribe(hub *Hub, c resp.Connection, args [][]byte) resp.Reply {
	channels := make([]string, len(args))
	for i, b := range args {
		channels[i] = string(b)
	}

	for _, channel := range channels {
		if subscribe0(hub, channel, c) {
			_ = c.Write(makeMsg(_subscribe, channel, int64(c.SubsCount())))
		}
	}
	return &reply.NoReply{}
}

// UnsubscribeAll removes the given connection from all subscribing channel
func UnsubscribeAll(hub *Hub, c resp.Connection) {
	channels := c.GetChannels()

	for _, channel := range channels {
		unsubscribe0(hub, channel, c)
	}

}

// UnSubscribe removes the given connection from the given channel
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
			_ = c.Write(makeMsg(_unsubscribe, channel, int64(c.SubsCount())))
		}
	}
	return &reply.NoReply{}
}

// Publish send msg to all subscribing client
func Publish(hub *Hub, args [][]byte) resp.Reply {
	if len(args) != 2 {
		return &reply.ArgNumErrReply{Cmd: "publish"}
	}
	channel := string(args[0])
	message := args[1]

	raw, ok := hub.subs.Get(channel)
	if !ok {
		return reply.MakeIntReply(0)
	}
	subscribers, _ := raw.(*list.LinkedList)
	subscribers.ForEach(func(i int, c interface{}) bool {
		client, _ := c.(resp.Connection)
		replyArgs := make([][]byte, 3)
		replyArgs[0] = messageBytes
		replyArgs[1] = []byte(channel)
		replyArgs[2] = message
		_ = client.Write(reply.MakeMultiBulkReply(replyArgs).ToBytes())
		return true
	})
	return reply.MakeIntReply(int64(subscribers.Len()))
}
