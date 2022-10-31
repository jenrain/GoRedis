
# GoRedis

## 简介

使用Go语言实现了一个简易版的单机Redis服务器，可以同时响应多个客户端的查询或者存储请求。

功能实现：

1.使用sync.Map实现内存数据库，可以并发安全地处理多个客户端的读写请求。

2.支持string、list、hash、set、sorted set数据结构。

3.服务器与客户端之间使用RESP协议进行通信。

4.支持AOF持久化以及AOF重写。

5.支持事务，实现了``watch``、``multi``、``exec``、``discard``命令。

6.支持发布/订阅模式。


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
    

## 核心功能说明

### AOF持久化和重写

如果配置文件中开启了AOF（appendonly yes），就会执行AOF持久化和AOF重写。

**AOF重写**

数据库在初始化的时候会调用``aof``包的``NewAofHandler``函数，该函数会从配置文件中读取aof持久化文件的路径和文件名，然就调用``handler.LoadAof``函数来处理aof文件。该函数首先会调用resp包中的``ParseStream``函数来解析文件中的指令，接着创建一个伪客户端来执行解析出来的所有命令，执行完之后，aof重写就完成了。

**AOF持久化**

AOF处理程序在初始化的时候会创建一个``handler.aofChan``通道，然后开启一个协程监听该通道，如果接收到值，就将通道中的指令落盘。

数据库的每一个DB都有一个``addAof``字段，该字段是一个函数。数据库在初始化时，会为每一个DB都初始化这个函数。当数据库执行写指令时，该函数会将当前的DB编号以及指令，封装到一个payload结构体中，然后发送到``handler.aofChan``通道中。

### 事务实现

**如何监听key是否改变**

主要是基于版本号来实现，每个``DB``中维护了一个``versionMap``，是``map[string]int``结构，用来储存每个key的版
本号，每个命令体都内含三个函数：``prepare``、``exec``、``undo``，分别用于提取出被写和被读的key、执行命令以及
命令回滚，每条命令在执行前都会调用prepare函数，提取出被写的key，然后更新版本号。

**watch的实现**

``watch``是一个乐观锁，每个客户端结构体都有一个储存被监视key的map，如果执行了``watch``命令，程序会把
``watch``后面的key都储存进一个``watchMap``（map[string]int，储存key的名字和版本号），在执行事务前，程序会检查watchMap中的所有key，如果
某个key的版本发生了变化，会直接结束事务。

**事务开始**

每个客户端结构体都有一个标记事务状态的变量：``multiState``，事务开始时，程序会将这个变量标记为``true``。

**事务开始之后**

每个客户端结构体都有一个储存事务命令的队列：``queue``，每条命令在执行前都会先判断当前的事务状态，
即判断``multiState``是否为``true``，如果为``true``，那么不会立即执行这条命令，而是将它储存进事务队列中。
每个客户端结构体都有一个储存事务执行时出现错误的切片：``txErrors``，在将命令储存进事务队列之前，会先判断这条
命令是否存在或者是否有语法错误，如果有，就先将``error``放入``txErrors``切片中，然后再将命令入队。

**取消事务**

将事务状态``multiState``设置为``false``，再将事务队列清空即可。

**执行事务**

1. 首先判断``txErrors``切片中是否有值，如果该切片中有值，说明命令有错误，那么会直接放弃事务。
2. 接着将每条命令中被写的key都取出来，然后与``watchMap``中的版本号做比较，如果版本号发生变化，
  会直接结束事务。
3. 接着开始正式执行事务队列中的命令，一边执行命令一边取出每条命令对应的回滚函数，存进一个切片``undoCmdLines``中，
  如果执行命令的过程中出现了语义错误，就停止执行后面的命令，逆向执行``undoCmdLines``中的回滚函数即可。

**回滚**

在执行每一条命令之前，程序会先根据这条命令生成对应的回滚命令，然后存到一个切片里面，如果中间某条命令执行失败，就根据切片中的命令进行回滚。
回滚命令的生成与原命令是对应的，比如SET的回滚命令就是DEL，RPUSH的回滚命令就是RPOP，如果某条命令的执行流程较为复杂，那么会执行一个万能回滚命令：``rollbackGivenKeys``，该函数比较简单粗暴，它会将key直接删除，然后将原来key对应的数据重新set进数据库。

### 发布/订阅实现

**主要的数据结构**

客户端维护的信息：客户端维护了一个``map``，该``map``的结构是``map[string]true``，表示该客户端订阅了哪些频道。（下面简称为``clientMap``）
服务器维护的信息：服务器的``pubsub``包中维护了一个``subs map``，该``map``是``map[string]*List``结构，List中存的是
客户端结构体，储存的是频道和频道的订阅者链表。（下面简称为``serverMap``）。

**订阅频道**

首先向``clientMap``中添加这个频道，然后向``serverMap``中添加相关信息：
 	1. 如果``serverMap``中存在该频道，那么取出``client``链表，再将当前``client`添加进去。
 	2. 如果``serverMap``中不存在该频道，那么创建``client``链表，再将当前``client``添加进去。

**退订频道**

首先向``clientMap``中删除这个频道，然后向``serverMap``中删除相关信息：

1. 如果``serverMap``中存在该频道，那么取出``client``链表，再将当前``client``删除。
2. 如果``serverMap``中不存在该频道，直接向客户端返回错误信息。

**发送消息**

从``serverMap``中取出对应的频道以及频道的订阅者链表，然后遍历链表，依次向订阅者发送消息。

## 实现命令

### Ping命令：

``*1\r\n$4\r\nPing\r\n``

### KEYS命令：

**DEL City3**

``*2\r\n$3\r\nDEL\r\n$5\r\nCity3\r\n``

KEYS *

```*2\r\n$4\r\nKEYS\r\n$1\r\n*\r\n```

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

**FlushDB**

``*1\r\n$7\r\nFlushDB\r\n``

### 事务命令：

**监听key**
``*2\r\n$5\r\nWatch\r\n$5\r\nCity1\r\n``
**开始事务**
``*1\r\n$5\r\nMulti\r\n``
**执行事务**
``*1\r\n$4\r\nExec\r\n``
**放弃事务**
``*1\r\n$7\r\nDiscard\r\n``

### 发布订阅命令

**订阅一个或多个频道 Subscribe Chan1**
``*2\r\n$9\r\nSubscribe\r\n$5\r\nChan1\r\n``
**向一个频道发消息 Publish Chan1**
``*3\r\n$7\r\nPublish\r\n$5\r\nChan1\r\n$5\r\nhello\r\n``
**退订一个或多个频道 UnSubscribe Chan1**
``*2\r\n$11\r\nUnSubscribe\r\n$5\r\nChan1\r\n``

### 五大数据结构相关命令：

#### 字符串对象：

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

#### 列表对象：

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

#### 哈希对象：

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

#### 集合对象：

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

#### 有序集合对象：

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

