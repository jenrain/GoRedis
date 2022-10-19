package reply

// 定义一些固定的回复

// PongReply 对Ping的回复
type PongReply struct{}

var pongBytes = []byte("+PONG\r\n")

func (p PongReply) ToBytes() []byte {
	return pongBytes
}

func MakePongReply() *PongReply {
	return &PongReply{}
}

// OKReply OK回复
type OKReply struct{}

var okBytes = []byte("+OK\r\n")

func (O OKReply) ToBytes() []byte {
	return okBytes
}

// 在包本地创建一个常量，回复的时候不必重新创建
var theOkReply = new(OKReply)

func MakeOkReply() *OKReply {
	return theOkReply
}

// NullBulkReply 空字符串回复
type NullBulkReply struct{}

var nullBulkBytes = []byte("$-1\r\n")

func (n NullBulkReply) ToBytes() []byte {
	return nullBulkBytes
}

func MakeNullBulkReply() *NullBulkReply {
	return &NullBulkReply{}
}

// EmptyMultiBulkReply 空数组回复
type EmptyMultiBulkReply struct{}

var emptyMultiBulkBytes = []byte("*\r\n")

func (e EmptyMultiBulkReply) ToBytes() []byte {
	return emptyMultiBulkBytes
}

// NoReply 空回复
type NoReply struct{}

var noBytes = []byte("")

func (n NoReply) ToBytes() []byte {
	return noBytes
}
