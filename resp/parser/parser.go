package parser

import (
	"GoRedis/interface/resp"
	"GoRedis/lib/logger"
	"GoRedis/resp/reply"
	"bufio"
	"errors"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
)

/*--- Redis解析器 ---*/

// Payload 客户端发送的指令
type Payload struct {
	// 客户端与服务器互相发送的数据格式是一样的
	Data resp.Reply
	Err  error
}

// 解析器的状态
type readState struct {
	// 是否在解析多行数据
	readingMultiLine bool
	// 正在解析的指令应该有几个参数
	expectedArgsCount int
	// 解析的类型
	msgType byte
	// 用户传过来的数据本身
	args [][]byte
	// 数据块的长度（$符号后面的数字）
	bulkLen int64
}

// 判断是否已经解析结束
func (s *readState) finished() bool {
	return s.expectedArgsCount > 0 && len(s.args) == s.expectedArgsCount
}

// ParseStream 协议层对外提供的API
func ParseStream(reader io.Reader) <-chan *Payload {
	ch := make(chan *Payload)
	// 异步解析命令
	// 一个用户创建一个解析器
	go parse0(reader, ch)
	// 业务层会一直监听这个channel，不阻塞
	return ch
}

// 进行解析的函数
func parse0(reader io.Reader, ch chan<- *Payload) {
	defer func() {
		// 防止循环中有错误直接退出
		if err := recover(); err != nil {
			logger.Error(string(debug.Stack()))
		}
	}()
	bufReader := bufio.NewReader(reader)
	var state readState
	var err error
	var msg []byte
	for {
		// read line
		var ioErr bool
		// 先读取一行的数据
		msg, ioErr, err = readLine(bufReader, &state)
		if err != nil {
			// 出现IO错误
			if ioErr {
				ch <- &Payload{
					Err: err,
				}
				close(ch)
				return
			}
			// 不是IO错误就是协议错误
			ch <- &Payload{
				Err: err,
			}
			// 将状态清空，跳过
			state = readState{}
			continue
		}

		// 解析命令的开头
		if !state.readingMultiLine {
			// 单行解析和多行解析最开始都走这个分支
			// 单行解析一次性就能解析完成
			// 多行解析是先取出*号后面的数字，再标记为多行解析，之后就不走这条分支
			// 开头是*号的多行命令
			if msg[0] == '*' {
				// multi bulk reply
				err = parseMultiBulkHeader(msg, &state)
				if err != nil {
					ch <- &Payload{
						Err: errors.New("protocol error: " + string(msg)),
					}
					state = readState{} // reset state
					continue
				}
				// *号后面的数字为0，不能继续解析，不用返回错误，跳过即可
				if state.expectedArgsCount == 0 {
					ch <- &Payload{
						Data: &reply.EmptyMultiBulkReply{},
					}
					state = readState{}
					continue
				}
				// 开头是$号的多行命令
				// $4\r\nPING\r\n
			} else if msg[0] == '$' {
				err = parseBulkHeader(msg, &state)
				// 协议错误
				if err != nil {
					ch <- &Payload{
						Err: errors.New("protocol error: " + string(msg)),
					}
					state = readState{}
					continue
				}
				// $-1\r\n	空指令
				if state.bulkLen == -1 {
					ch <- &Payload{
						Data: &reply.NullBulkReply{},
					}
					state = readState{}
					continue
				}
			} else {
				// 单行指令 一次就能解析完
				result, err := parseSingleLineReply(msg)
				ch <- &Payload{
					Data: result,
					Err:  err,
				}
				state = readState{}
				continue
			}
		} else {
			// 解析多行命令的命令体
			err = readBody(msg, &state)
			// 协议错误
			if err != nil {
				ch <- &Payload{
					Err: errors.New("protocol error: " + string(msg)),
				}
				state = readState{} // reset state
				continue
			}
			// 解析结束
			if state.finished() {
				var result resp.Reply
				// 多行命令
				if state.msgType == '*' {
					result = reply.MakeMultiBulkReply(state.args)
					// 单行命令
				} else if state.msgType == '$' {
					result = reply.MakeBulkReply(state.args[0])
				}
				ch <- &Payload{
					Data: result,
					Err:  err,
				}
				state = readState{}
			}
		}
	}
}

// *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
// 读取一行数据，以\r\n结尾或者以$后面的数字为长度
func readLine(bufReader *bufio.Reader, state *readState) ([]byte, bool, error) {
	// 两种情况
	// 1.未读到$符号，依照\r\n切分
	// 2.读到$符号，按照$后面的数字读取字符个数
	var msg []byte
	var err error
	// 1.依照\r\n切分
	if state.bulkLen == 0 {
		msg, err = bufReader.ReadBytes('\n')
		if err != nil {
			return nil, true, err
		}
		if len(msg) == 0 || msg[len(msg)-2] != '\r' {
			return nil, false, errors.New("protocol error: " + string(msg))
		}
	} else { // 2.依照$符号后面的数字切分
		msg = make([]byte, state.bulkLen+2)
		_, err = io.ReadFull(bufReader, msg)
		if err != nil {
			return nil, true, err
		}
		if len(msg) == 0 ||
			msg[len(msg)-2] != '\r' ||
			msg[len(msg)-1] != '\n' {
			return nil, false, errors.New("protocol error: " + string(msg))
		}
		state.bulkLen = 0
	}
	return msg, false, nil
}

// 如果是多行命令，取出*号后面的数字
func parseMultiBulkHeader(msg []byte, state *readState) error {
	var err error
	var expectedLine uint64
	// 取出数字
	expectedLine, err = strconv.ParseUint(string(msg[1:len(msg)-2]), 10, 32)
	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if expectedLine == 0 {
		state.expectedArgsCount = 0
		return nil
	} else if expectedLine > 0 {
		// 初始化多行命令的state结构体
		state.msgType = msg[0]
		state.readingMultiLine = true
		state.expectedArgsCount = int(expectedLine)
		state.args = make([][]byte, 0, expectedLine)
		return nil
	} else {
		return errors.New("protocol error: " + string(msg))
	}
}

// $4\r\nPING\r\n
// 取出$号后面的数字
// 也属于多行解析
func parseBulkHeader(msg []byte, state *readState) error {
	var err error
	state.bulkLen, err = strconv.ParseInt(string(msg[1:len(msg)-2]), 10, 64)
	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if state.bulkLen == -1 {
		return nil
	} else if state.bulkLen > 0 {
		state.msgType = msg[0]
		state.readingMultiLine = true
		state.expectedArgsCount = 1
		state.args = make([][]byte, 0, 1)
		return nil
	} else {
		return errors.New("protocol error: " + string(msg))
	}
}

// +OK\r\n -err\r\n	:5\r\n
// 解析单行回复
func parseSingleLineReply(msg []byte) (resp.Reply, error) {
	// 先把\r\n切掉
	str := strings.TrimSuffix(string(msg), "\r\n")
	var result resp.Reply
	switch msg[0] {
	case '+': // status reply
		result = reply.MakeStatusReply(str[1:])
	case '-': // err reply
		result = reply.MakeErrReply(str[1:])
	case ':': // int reply
		val, err := strconv.ParseInt(str[1:], 10, 64)
		if err != nil {
			return nil, errors.New("protocol error: " + string(msg))
		}
		result = reply.MakeIntReply(val)
	}
	return result, nil
}

// $3\r\nSET\r\n$3\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
// 解析命令本体
func readBody(msg []byte, state *readState) error {
	line := msg[0 : len(msg)-2]
	var err error
	// 如果读到的是bulkLen，就设置bulkLen
	if line[0] == '$' {
		// bulk reply
		state.bulkLen, err = strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return errors.New("protocol error: " + string(msg))
		}
		if state.bulkLen <= 0 { // null bulk in multi bulks
			state.args = append(state.args, []byte{})
			state.bulkLen = 0
		}
	} else {
		// 读到的是命令本体
		state.args = append(state.args, line)
	}
	return nil
}
