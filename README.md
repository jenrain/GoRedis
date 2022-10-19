
# GoRedis

## 简介

使用Go语言实现了一个简易版的单机Redis服务器，可以同时响应多个客户端的查询或者存储请求。

功能实现：

1.使用sync.Map实现内存数据库，可以并发安全地处理多个客户端的读写请求。

1.支持string、list、hash、set、sorted set数据结构。

2.服务器与客户端之间使用RESP协议进行通信。

3.支持AOF持久化以及AOF重写。


## 一个客户端命令的执行步骤

**1.TCP服务器：**

- 监听配置文件指定的端口，默认是63791. 首先创建两个通道：sigChan和closeChan，sigChan用于接收来自系统的关闭信号. 服务器开启两个协程，分别用于监听sigChan和closeChan，如果sigChan通道有值，就说明系统发送了关系信号，直接向closeChan通道发送值，服务器收到closeChan的通知，正常关闭服务器.
- 服务器接收到来自客户端的连接之后，开启一条协程去调用解析层的handler.Handle(ctx, conn)函数.

**2.协议解析：**

- 首先Handler函数会将传进来的连接 -- conn参数，包装成一个connection.Connection，该结构体组合了一个conn net.Conn字段，并加上其它信息：当前数据库编号、实现超时关闭功能的等待组、互斥锁.
- 然后调用解析层的ParseStream函数，该函数会开启一个协程对conn中的参数进行解析，并返回一个无缓冲通道ch，解析好的命令就包装成PayLoad结构体（含Data和Err两个字段）发送到通道里.
- 同时外层会一直监听ch，如果取到值，首先处理错误，然后调用handler.db.Exec(client, r.Args)开始执行解析好的命令，有错误就调用client.Write回写给客户端.

**3.内存数据库：**

- 首先数据库引擎层的Exec函数会切换到当前客户端指定的数据库，如果没有指定，默认就是0号数据库.
- 然后调用数据库层的db.Exec(client, args)函数开始真正在内存数据库中执行命令.
- 在db.Exec函数中，首先从命令列表cmdTable（map[string]*command，其中command包含一个命令的执行函数有以及该命令的参数数量）中取出对应的命令，然后执行.

**4. 内存数据库的数据结构：**

- 内存数据库是sync.Map实现的，该map的kay是string类型的命令名称，value是*database.DataEntity类型的值，其中DataEntity是一个结构体，里面包含一个interface{}.
- 然后内存数据库会根据指定的命令，将对应的数据结构存入map中；如果是读命令，首先根据key获取到对应的*database.DataEntity，然后使用类型断言转换成对应的数据结构.
- 一共有5种数据结构：
    1. string：go语言内置的string类型实现.
    2. list：双向链表实现.
    3. hash：sync.Map实现.
    4. set：sync.Map实现.
    5. sortedset：sync.Map和跳表共同实现.
    
**5.AOF持久化和重写：**

- 配置文件中开启了AOF（appendonly yes）就会执行AOF持久化和AOF重写.
- 数据库引擎中有一个*aof.AofHandler字段，引擎本身在初始化的时候，就会调用NewAofHandler初始化*aof.AofHandler字段，如果可以读取到appendonly.aof文件，那么就会调用LoadAof()函数执行aof重写，重写会创建一个伪客户端，然后把aof文件中的命令都执行一遍.
- 每个写命令和删除命令在执行的时候都会调用AddAof函数，该函数会将命令和数据库编号封装成payload并发送到handler.aofChan中.
- 同时，在执行NewAofHandler的时候，程序会开启一个协程用来执行handler.handlerAof()，该函数监听handler.aofChan，并将命令落盘.


## 实现命令

### Ping命令：

``*1\r\n$4\r\nPing\r\n``

### KEYS命令：

**DEL City3**

``*2\r\n$3\r\nDEL\r\n$5\r\nCity3\r\n``

KEYS *

```*2\r\n$4\r\nKEYS\r\n$1\r\n*\r\n```

**Flushdb**

``*1\r\n$7\r\nFlushdb\r\n``

**Exists City1**

**Exists City3**

``*2\r\n$6\r\nExists\r\n$5\r\nCity1\r\n``

``*2\r\n$6\r\nExists\r\n$5\r\nCity3\r\n``

**Type City1**

``*2\r\n$4\r\nTYPE\r\n$5\r\nCity1\r\n``

**Rename City1 City3**

``*3\r\n$6\r\nRename\r\n$5\r\nCity1\r\n$5\r\nCity3\r\n``

**RenameNX City3 City1**

``*3\r\n$8\r\nRenamenx\r\n$5\r\nCity3\r\n$5\r\nCity1\r\n``


### 字符串对象：

**Set City1 Shanghai**

``*3\r\n$3\r\nSET\r\n$5\r\nCity1\r\n$8\r\nShanghai\r\n``

**Get City1**

``*2\r\n$3\r\nGET\r\n$5\r\nCity1\r\n``

**SetNx City1 Ningbo**

**SetNx City4 xian**

``*3\r\n$5\r\nSETNX\r\n$5\r\nCity1\r\n$6\r\nNingbo\r\n``

``*3\r\n$5\r\nSETNX\r\n$5\r\nCity4\r\n$4\r\nxian\r\n``

**GetSet City1 Beijing**

``*3\r\n$6\r\nGETSET\r\n$5\r\nCity1\r\n$7\r\nBeijing\r\n``

**StrLen City1**

**Strlen City2**

``*2\r\n$6\r\nStrLen\r\n$5\r\nCity1\r\n``

``*2\r\n$6\r\nStrLen\r\n$5\r\nCity2\r\n``

### 列表对象：

**LPush fruit apple banana peach**

``*5\r\n$5\r\nLPUSH\r\n$5\r\nfruit\r\n$5\r\napple\r\n$6\r\nbanana\r\n$5\r\npeach\r\n``

**LPushX fruit cherry**

**LPushX food pork**

``*3\r\n$6\r\nLPushX\r\n$5\r\nfruit\r\n$6\r\ncherry\r\n``

``*3\r\n$6\r\nLPushX\r\n$4\r\nfood\r\n$4\r\npork\r\n``

**RPush food pork beef mutton**

``*5\r\n$5\r\nRPush\r\n$4\r\nfood\r\n$4\r\npork\r\n$4\r\nbeef\r\n$6\r\nmutton\r\n``

**RPushX food chicken**

**RPushX drink cola**

``*3\r\n$6\r\nRPushX\r\n$4\r\nfood\r\n$7\r\nchicken\r\n``

``*3\r\n$6\r\nRPushX\r\n$5\r\ndrink\r\n$4\r\ncola\r\n``

**LPop fruit**

``*2\r\n$4\r\nLPop\r\n$5\r\nfruit\r\n``

**RPop fruit**

``*2\r\n$4\r\nRPop\r\n$5\r\nfruit\r\n``

**LRem fruit 2 beef**

``*4\r\n$4\r\nLRem\r\n$5\r\nfruit\r\n$1\r\n2\r\n$4\r\nbeef\r\n``

**LLen fruit**

``*2\r\n$4\r\nLLen\r\n$5\r\nfruit\r\n``

**LIndex fruit 2**

``*3\r\n$6\r\nLIndex\r\n$5\r\nfruit\r\n$1\r\n2\r\n``

**LSet fruit 1 cherry**

``*4\r\n$4\r\nLSet\r\n$5\r\nfruit\r\n$1\r\n1\r\n$6\r\ncherry\r\n``

**LRange fruit 0 3**

``*4\r\n$6\r\nLRange\r\n$5\r\nfruit\r\n$1\r\n0\r\n$1\r\n3\r\n``

### 哈希对象：
**HSet age lihua 18**

``*4\r\n$4\r\nHSet\r\n$3\r\nage\r\n$5\r\nlihua\r\n$2\r\n18\r\n``

**HSetNX age xiaoming 20**

**HSetNX phone lihua 18700000001**

``*4\r\n$6\r\nHSetNX\r\n$3\r\nage\r\n$8\r\nxiaoming\r\n$2\r\n20\r\n``

``*4\r\n$6\r\nHSetNX\r\n$5\r\nphone\r\n$5\r\nlihua\r\n$11\r\n18700000001\r\n``

**HGet age lihua**

``*3\r\n$4\r\nHGet\r\n$3\r\nage\r\n$5\r\nlihua\r\n``

**HExists age lihua**

**HExists age xiaoming**

``*3\r\n$7\r\nHExists\r\n$3\r\nage\r\n$5\r\nlihua\r\n``

``*3\r\n$7\r\nHExists\r\n$3\r\nage\r\n$8\r\nxiaoming\r\n``

**HDel age lihua**

``*3\r\n$4\r\nHDel\r\n$3\r\nage\r\n$5\r\nlihua\r\n``

**HLen age**

``*2\r\n$4\r\nHLen\r\n$3\r\nage\r\n``

**HMSet age xiaoming 20 xiaohai 21 xiaohong 22**

``*8\r\n$5\r\nHMSet\r\n$3\r\nage\r\n$8\r\nxiaoming\r\n$2\r\n20\r\n$7\r\nxiaohai\r\n$2\r\n21\r\n$8\r\nxiaohong\r\n$2\r\n22\r\n``

**HKeys age**

``*2\r\n$5\r\nHKeys\r\n$3\r\nage\r\n``

**HVals age**

``*2\r\n$5\r\nHVals\r\n$3\r\nage\r\n``

**HGetAll age**

``*2\r\n$7\r\nHGetAll\r\n$3\r\nage\r\n``

### 集合对象：

**SAdd fruit1 banana apple**

**SAdd fruit2 apple cherry peach**

``*4\r\n$4\r\nSAdd\r\n$6\r\nfruit1\r\n$6\r\nbanana\r\n$5\r\napple\r\n``

``*5\r\n$4\r\nSAdd\r\n$6\r\nfruit2\r\n$5\r\napple\r\n$6\r\ncherry\r\n$5\r\npeach\r\n``

**SIsMember fruit1 apple**

**SIsMember fruit1 cherry**

``*3\r\n$9\r\nSIsMember\r\n$6\r\nfruit1\r\n$5\r\napple\r\n``

``*3\r\n$9\r\nSIsMember\r\n$6\r\nfruit1\r\n$6\r\ncherry\r\n``

**SRem fruit2 peach**

``*3\r\n$4\r\nSRem\r\n$6\r\nfruit2\r\n$5\r\npeach\r\n``

**SCard fruit1**

``*2\r\n$5\r\nSCard\r\n$6\r\nfruit1\r\n``

**SMembers fruit1**

**SMembers fruit2**

``*2\r\n$8\r\nSMembers\r\n$6\r\nfruit1\r\n``

``*2\r\n$8\r\nSMembers\r\n$6\r\nfruit2\r\n``

**SInter fruit1 fruit2**

``*3\r\n$6\r\nSInter\r\n$6\r\nfruit1\r\n$6\r\nfruit2\r\n``

**SUnion fruit1 fruit2**

``*3\r\n$6\r\nSUnion\r\n$6\r\nfruit1\r\n$6\r\nfruit2\r\n``

**SDiff fruit1 fruit2**

``*3\r\n$5\r\nSDiff\r\n$6\r\nfruit1\r\n$6\r\nfruit2\r\n``

### 有序集合对象：

**ZAdd meet pork 18 beef 20 mutton 22**

``*8\r\n$4\r\nZAdd\r\n$4\r\nmeet\r\n$2\r\n18\r\n$4\r\npork\r\n$2\r\n20\r\n$4\r\nbeef\r\n$2\r\n22\r\n$6\r\nmutton\r\n``

**ZScore meet pork**

``*3\r\n$6\r\nZScore\r\n$4\r\nmeet\r\n$4\r\npork\r\n``

**ZRank meet beef**

``*3\r\n$5\r\nZRank\r\n$4\r\nmeet\r\n$4\r\nbeef\r\n``

**ZCount meet 0 30**

**ZCount meet 19 24**

``*4\r\n$6\r\nZCount\r\n$4\r\nmeet\r\n$1\r\n0\r\n$2\r\n30\r\n``

``*4\r\n$6\r\nZCount\r\n$4\r\nmeet\r\n$2\r\n19\r\n$2\r\n24\r\n``

**ZCard meet**

``*2\r\n$5\r\nZCard\r\n$4\r\nmeet\r\n``

**ZRange meet 0 1**

**ZRange meet 0 1 WITHSCORES**

``*4\r\n$6\r\nZRange\r\n$4\r\nmeet\r\n$1\r\n0\r\n$1\r\n1\r\n``

``*5\r\n$6\r\nZRange\r\n$4\r\nmeet\r\n$1\r\n0\r\n$1\r\n1\r\n$10\r\nWITHSCORES\r\n``

**ZRem meet port**

``*3\r\n$4\r\nZRem\r\n$4\r\nmeet\r\n$4\r\npork\r\n``

**ZRemRangeByScore meet 17 19**

``*4\r\n$16\r\nZRemRangeByScore\r\n$4\r\nmeet\r\n$2\r\n17\r\n$2\r\n19\r\n``

**ZRemRangeByRank meet 0 1**

``*4\r\n$15\r\nZRemRangeByRank\r\n$4\r\nmeet\r\n$1\r\n0\r\n$1\r\n1\r\n``


