package reply

/*-- 自定义回复 --*/

import (
	"GoRedis/interface/resp"
	"bytes"
	"strconv"
)

var (
	nullBulkReplyBytes = []byte("$-1")
	CRLF               = "\r\n"
)

// BulkReply 单行命令的回复
type BulkReply struct {
	// 待回复的内容
	Arg []byte
}

func (b *BulkReply) ToBytes() []byte {
	if len(b.Arg) == 0 {
		return nullBulkReplyBytes
	}
	// $7\r\nmessage\r\n
	return []byte("$" + strconv.Itoa(len(b.Arg)) + CRLF + string(b.Arg) + CRLF)
}

func MakeBulkReply(arg []byte) *BulkReply {
	return &BulkReply{Arg: arg}
}

// MultiBulkReply 多行命令的回复
type MultiBulkReply struct {
	Args [][]byte
}

func (r *MultiBulkReply) ToBytes() []byte {
	argLen := len(r.Args)
	var buf bytes.Buffer
	// *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
	buf.WriteString("*" + strconv.Itoa(argLen) + CRLF)
	for _, arg := range r.Args {
		if arg == nil {
			buf.WriteString(string(nullBulkReplyBytes) + CRLF)
		} else {
			buf.WriteString("$" + strconv.Itoa(len(arg)) + CRLF + string(arg) + CRLF)
		}
	}
	return buf.Bytes()
}

func MakeMultiBulkReply(arg [][]byte) *MultiBulkReply {
	return &MultiBulkReply{Args: arg}
}

// MultiRawReply 多条回复，比如事务执行完毕的回复
type MultiRawReply struct {
	Replies []resp.Reply
}

func MakeMultiRawReply(replies []resp.Reply) *MultiRawReply {
	return &MultiRawReply{
		Replies: replies,
	}
}

func (r *MultiRawReply) ToBytes() []byte {
	argLen := len(r.Replies)
	var buf bytes.Buffer
	buf.WriteString("*" + strconv.Itoa(argLen) + CRLF)
	for _, arg := range r.Replies {
		buf.Write(arg.ToBytes())
	}
	return buf.Bytes()
}

// StatusReply 回复一个状态
type StatusReply struct {
	Status string
}

func MakeStatusReply(status string) *StatusReply {
	return &StatusReply{
		Status: status,
	}
}

func (r *StatusReply) ToBytes() []byte {
	// +OK\r\n
	return []byte("+" + r.Status + CRLF)
}

// IntReply 数字回复
type IntReply struct {
	Code int64
}

func MakeIntReply(code int64) *IntReply {
	return &IntReply{
		Code: code,
	}
}

func (r *IntReply) ToBytes() []byte {
	// :5\r\n
	return []byte(":" + strconv.FormatInt(r.Code, 10) + CRLF)
}

// ErrorReply 错误回复接口
type ErrorReply interface {
	Error() string
	ToBytes() []byte
}

// StandardErrReply 标准错误回复
type StandardErrReply struct {
	Status string
}

func (r *StandardErrReply) ToBytes() []byte {
	// -Err ... \r\n
	return []byte("-" + r.Status + CRLF)
}

func (r *StandardErrReply) Error() string {
	return r.Status
}

func MakeErrReply(status string) *StandardErrReply {
	return &StandardErrReply{
		Status: status,
	}
}

// IsErrorReply 判断是否是错误回复
func IsErrorReply(reply resp.Reply) bool {
	// 判断字节数组的第一个元素是不是减号
	return reply.ToBytes()[0] == '-'
}

// QueuedReply 执行事务时，命令的入队回复
type QueuedReply struct{}

var queuedBytes = []byte("+QUEUED\r\n")

func (r *QueuedReply) ToBytes() []byte {
	return queuedBytes
}

var theQueuedReply = new(QueuedReply)

func MakeQueuedReply() *QueuedReply {
	return theQueuedReply
}
