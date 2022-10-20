package database

import (
	"GoRedis/interface/resp"
	"GoRedis/resp/reply"
	"strings"
)

// Watch 开始监听给定的key
func Watch(db *DB, conn resp.Connection, args [][]byte) resp.Reply {
	// 获取储存事务key的map
	watching := conn.GetWatching()
	for _, bkey := range args {
		key := string(bkey)
		// 事务key的值就是该key的版本号
		watching[key] = db.GetVersion(key)
	}
	return reply.MakeOkReply()
}

// 返回key的版本号
func execGetVersion(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	ver := db.GetVersion(key)
	return reply.MakeIntReply(int64(ver))
}

func init() {
	RegisterCommand("GetVer", execGetVersion, readAllKeys, nil, 2)
}

// 判断正在监视的key版本号有没有发生变化
func isWatchingChanged(db *DB, watching map[string]uint32) bool {
	for key, ver := range watching {
		currentVersion := db.GetVersion(key)
		if ver != currentVersion {
			return true
		}
	}
	return false
}

// StartMulti 开始事务
func StartMulti(conn resp.Connection) resp.Reply {
	// 判断之前有没有执行过multi命令
	if conn.InMultiState() {
		// 事务不能嵌套执行
		return reply.MakeErrReply("ERR MULTI calls can not be nested")
	}
	conn.SetMultiState(true)
	return reply.MakeOkReply()
}

// EnqueueCmd puts command line into `multi` pending queue
// 将在事务进行时执行的命令入队
func EnqueueCmd(conn resp.Connection, cmdLine [][]byte) resp.Reply {
	cmdName := strings.ToLower(string(cmdLine[0]))
	cmd, ok := cmdTable[cmdName]
	if !ok {
		err := reply.MakeErrReply("ERR unknown command '" + cmdName + "'")
		// 储存事务执行时产生的错误：无效的命令
		conn.AddTxError(err)
		return err
	}
	if cmd.prepare == nil {
		err := reply.MakeErrReply("ERR command '" + cmdName + "' cannot be used in MULTI")
		// 储存事务执行时产生的错误：该命令不能在事务开始时被执行
		conn.AddTxError(err)
		return err
	}
	// 验证参数个数是否正确
	if !validateArity(cmd.arity, cmdLine) {
		err := reply.MakeArgNumErrReply(cmdName)
		// 储存事务执行时产生的错误：命令参数数量错误
		conn.AddTxError(err)
		return err
	}
	// 将命令正式入队
	conn.EnqueueCmd(cmdLine)
	// 向客户端回复命令已入队
	return reply.MakeQueuedReply()
}

// 开始执行事务队列中的命令
func execMulti(db *DB, conn resp.Connection) resp.Reply {
	// 没有开启事务
	if !conn.InMultiState() {
		return reply.MakeErrReply("ERR EXEC without MULTI")
	}
	defer conn.SetMultiState(false)
	// 事务队列中的命令有语法错误，事务被丢弃
	if len(conn.GetTxErrors()) > 0 {
		return reply.MakeErrReply("EXECABORT Transaction discarded because of previous errors.")
	}
	// 从事务队列中获取命令
	cmdLines := conn.GetQueuedCmdLine()
	// 正式开始执行事务
	return db.ExecMulti(conn, conn.GetWatching(), cmdLines)
}

// ExecMulti
// 原子性以及隔离性地执行事务命令
// watching中储存了事务开启后所有key对应的版本号
// cmdLines中储存了事务开启后入队的命令
func (db *DB) ExecMulti(conn resp.Connection, watching map[string]uint32, cmdLines []CmdLine) resp.Reply {
	// 可能会包含重复
	writeKeys := make([]string, 0)
	readKeys := make([]string, 0)
	for _, cmdLine := range cmdLines {
		cmdName := strings.ToLower(string(cmdLine[0]))
		cmd := cmdTable[cmdName]
		prepare := cmd.prepare
		write, read := prepare(cmdLine[1:])
		writeKeys = append(writeKeys, write...)
		readKeys = append(readKeys, read...)
	}
	// set watch
	watchingKeys := make([]string, 0, len(watching))
	for key := range watching {
		watchingKeys = append(watchingKeys, key)
	}
	//readKeys = append(readKeys, watchingKeys...)
	//db.RWLocks(writeKeys, readKeys)
	//defer db.RWUnLocks(writeKeys, readKeys)

	// 判断在事务中监视的key，现在的版本号有没有发生变化
	if isWatchingChanged(db, watching) {
		return reply.EmptyMultiBulkReply{}
	}
	// 执行
	results := make([]resp.Reply, 0, len(cmdLines))
	aborted := false
	undoCmdLines := make([][]CmdLine, 0, len(cmdLines))
	for _, cmdLine := range cmdLines {
		undoCmdLines = append(undoCmdLines, db.GetUndoLogs(cmdLine))
		//result := db.execWithLock(cmdLine)
		result := db.NormalExec(cmdLine)
		if reply.IsErrorReply(result) {
			aborted = true
			// don't roll back failed commands
			undoCmdLines = undoCmdLines[:len(undoCmdLines)-1]
			break
		}
		results = append(results, result)
	}
	if !aborted { //success
		db.addVersion(writeKeys...)
		return reply.MakeMultiRawReply(results)
	}
	// undo if aborted
	size := len(undoCmdLines)
	for i := size - 1; i >= 0; i-- {
		curCmdLines := undoCmdLines[i]
		if len(curCmdLines) == 0 {
			continue
		}
		for _, cmdLine := range curCmdLines {
			db.NormalExec(cmdLine)
		}
	}
	return reply.MakeErrReply("EXECABORT Transaction discarded because of previous errors.")
}

// DiscardMulti 取消事务
func DiscardMulti(conn resp.Connection) resp.Reply {
	if !conn.InMultiState() {
		return reply.MakeErrReply("ERR DISCARD without MULTI")
	}
	// 清楚事务队列
	conn.ClearQueuedCmd()
	conn.SetMultiState(false)
	return reply.MakeOkReply()
}

// GetUndoLogs return rollback commands
// 返回回滚的命令
func (db *DB) GetUndoLogs(cmdLine [][]byte) []CmdLine {
	cmdName := strings.ToLower(string(cmdLine[0]))
	cmd, ok := cmdTable[cmdName]
	if !ok {
		return nil
	}
	undo := cmd.undo
	if undo == nil {
		return nil
	}
	return undo(db, cmdLine[1:])
}

// GetRelatedKeys analysis related keys
func GetRelatedKeys(cmdLine [][]byte) ([]string, []string) {
	cmdName := strings.ToLower(string(cmdLine[0]))
	cmd, ok := cmdTable[cmdName]
	if !ok {
		return nil, nil
	}
	prepare := cmd.prepare
	if prepare == nil {
		return nil, nil
	}
	return prepare(cmdLine[1:])
}
