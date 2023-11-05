# 2022-MIT-6.824
6.824 MIT 分布式系统开发学习课程，目前已 all pass




# MapReduce 


### MapReduce 源码解析


mrcoordinator.go：
```
c.RunningTasks= make(chan MRTask, len(files)+1) // 将任务放入channel中 // 使用队列也可以
c.server() // 将Coordinator中的所有方法都设为rpc

// rpc函数介绍：
RequestTask() // 返回一个任务给worker
  if len(c.RunningTasks) == 0{
    // 任务都完成，则进行转换状态
    // 并非任务都完成
        // 如果发现有超时任务，则将任务放回到c.RunningTasks。
        // 如果没有发现有超时任务，则reply.TaskType = PHASE_WAITTING，让worker等待。
  }
  if c.phase == PHASE_FINISH {reply.TaskType = PHASE_FINISH}
    // worker收到PHASE_FINISH，也会退出
  // 如果c.RunningTasks不为空，那么
  task, ok := <-c.RunningTasks
  task.startTime = time.Now().Unix() // 设置任务的开始时间，用于检测是否超时

RequestTaskDone()
  // 判断是否超时，如果超时，就返回提交失败
  // 否则提交，即如下：
  task.status = DONE
  Rename // 将临时文件名修改为正式的文件名
```

worker.go：使用rpc从mrcoordinator获取任务，并执行相应的函数（函数的核心为map或reduce），然后使用rpc通知mrcoordinator提交任务。
```
ok := call("Coordinator.RequestTask", &args, &reply) // rpc获取任务，每个文件都是一个任务
// 根据任务类型，执行相应的函数，主要有如下函数
DoMapTask()
  // 读取文件内容，并输入到mapf中
  kva := mapf(Task.FileName, string(content)) // 解析文件内容，生成kv键值对，本项目中k为单词，v为1。
  // 将kv写入到对应的临时文件中，总共有ReduceNum个临时文件，即每个任务都会输出ReduceNum个文件。
  // 文件命名为`"tmp-worker-%d-%d-%d", workerId, mapId, reduceId`
  fmt.Fprintf(outFile, "%v\t%v\n", kv.Key, kv.Value) 
  // 通知执行完毕，mrcoordinator.go执行RequestTaskDone()
  ok := call("Coordinator.RequestTaskDone", &args, &reply)   

DoReduceTask()
  // 读取id为Task.TaskID的所有文件。这些文件由map生成  
  // 解析这些文件中的kv键值对，并将kv键值对放入kva中
  // 将kv键值对进行排序，然后将相同key的结果放进values中
  output := reducef(kva[i].Key, values) // key为kva[i].Key的所有value都放进values中，
      // reducef解析values，并将结果以kv的形式返回。本项目中只是简单地返回key和values中元素的个数。
  ok := call("Coordinator.RequestTaskDone", &args, &reply)
```
map函数和reduce函数：
```
map：解析文件内容，生成kv键值对。其中k为单词，v为任意你想获取的信息。本项目中k为单词，v为"1"。
相同k的v放进一个values中，得到[k,values]，输入到reduce中。下面我将[k,values]也简写为kv。
reduce：解析kv，并一个kv形式的结果。本项目中只是简单地返回key和values中元素的个数。
```
rpc
```
server()
  rpc.Register(c) // 将RPC服务注册到默认的HTTP服务器中
  rpc.HandleHTTP() 
  l, e := net.Listen("unix", sockname) 
    // Unix域套接字 网络地址是通过文件系统中一个不存在的路径来表示的
    //  Unix域套接字  数据传输在内核中完成，不需经过网络设备，所以更加高效
  go http.Serve(l, nil)

ok := call("Coordinator.RequestTask", &args, &reply)
  c, err := rpc.DialHTTP("unix", sockname)
  err = c.Call(rpcname, args, reply) // args, reply都是interface{}类型
```

数据结构：
参考：https://www.cnblogs.com/lawliet12/p/16972376.html




### MapReduce 源码其他内容


```
go build -buildmode=plugin ../mrapps/wc.go
```
- -buildmode=plugin: 这是一个编译选项，指定编译模式为插件模式。插件模式允许将Go程序编译为可加载的插件，可以在运行时动态加载和使用。我觉得应该就是编译成一个动态库。
- `go run -race`检测协程之间的数据竞争，一般在调试和测试时使用。
- 动态库A的生成使用了`-race`，如果B要加载A作为插件，那么B的编译是也要使用`-race`。
- B程序中如果有加载动态库A作为插件，那么B程序好像不能使用vscode进行调试。














### MapReduce理论解析




参考链接：
- 看本视频理解MapReduce：https://www.bilibili.com/video/BV1wt411v7ny/
- 本解析内容框架是参考：https://blog.csdn.net/qq_21794823/article/details/108348837
看完上面的视频，就很容易看懂下面视频，下面视频是用来补充理解：https://www.bilibili.com/video/BV12Y4y1373q/
- 本篇讲的很细，可用补充看不懂的内容：https://zhuanlan.zhihu.com/p/380602829



MapReduce出现的目的：设计一个架构，用于解析大量文件或大量网页。map和reduce的输入和输出都是kv结构：
- map：输入[文件名，文件内容]，将文件拆解成多个[key,value]。
- shuffle：处理map拆解出来的[key,value]，（map时）进行排序、分区、combiner、写入磁盘，（reduce时）拉取远程的磁盘分区数据、排序、分组产生[key,valueList]。
- reduce：将[key,valueList]输入到reduce中。


shuffle在MapReduce中的位置如下所示（本图为Hadoop中的MapReduce的实现）：可以看出shuffle是map和reduce的中间过程。
![](https://img2023.cnblogs.com/blog/1617829/202310/1617829-20231031220044187-318197955.png)
具体我们以统计单词数量为例，MapReduce的shuffle被解析为如下图所示：
![](https://img2023.cnblogs.com/blog/1617829/202310/1617829-20231031214547084-1293876093.png)
结合上述两图，解析得到如下内容：
- 首先会将map输出的结果放入到buffer（环形缓冲区）中。
- Spill 阶段：当buffer中的数据超过buffer的80%以后，就会将buffer中的数据进行分区，并将分区内的数据进行排序（**快速排序**）。然后如果有combiner，则将会先进行combiner操作，如将多个“红 1”合并为“红 3”。最后将分区的数据写入到临时文件中。写入到临时文件的过程称为spill（溢出到磁盘）。
可以看出使用了combiner操作以后，所需的磁盘空间就变少，所以combiner相当于对数据做了压缩。【注】combiner必须不影响终结果，比如你要计算平均数时，就不能计算每个分区的平均数，最后加起来。
- Merge 阶段：把所有溢出的临时文件进行一次合并操作（**归并排序**），合并以后还可以进行一次combiner操作，最后将合并的文件写入磁盘。Merge 阶段用于确保一个MapTask最终只产生一个中间数据文件。
- 拉去分区数据： 每个Reduce会远程复制一个分区的所有文件，如果内存不够，则会将拉去到的文件存在磁盘中。远程复制数据的同时，会将分区数据进行排序（**归并排序**），然后进行分组。
- 分组：多个[红,1]分组为[红,[1,1,1...]]，这个[key,valueList]形式就是reduce的输入。






### 其他MapReduce知识

**MapReduce 和spark的区别？**
答：MapReduce 需要大量的读写磁盘的操作，所以速度慢。spark会尽量使用内存，所以速度快。


**只是用map行吗**
答：可以，就是不需要综合分析多个文件产生的多个相同分区的数据，那么就不需要reduce。










#  raft


### raft源码解析

领导者选举和日志复制：
```
ticker()：使用select……case，实现定时操作
  // 心跳超时，且为leader 
  rf.SendAppendEntries()
  rf.heartbeatTimer.Reset(100 * time.Millisecond)
  
  // 选举超时，即follower一段时间没有收到心跳，则成为candidate，并发送投票请求
  rf.SwitchRole(ROLE_Candidate)
  rf.StartElection()
  

rf.SendAppendEntries()：用于同步日志或发送心跳给follower节点。当follower中已经复制了leader的所有日志时，会发送一个日志体为空的rpc，这就是心跳。
  if server == rf.me { // 如果是当前节点，就不发送数据
	continue
  }
  go func(server int) {
    // 判断此时是否leader，不是，则return
    // 依据`matchIndex`判断follower是否有缺少的日志
        // 如果缺少日志在快照中，则将整个快照发送给follower，即
            go rf.SendInstallSnapshot(server) 
        // 在log中，就从log中发送
    ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

    if reply.Term > args.Term { // reply.Term对应的节点成功当选了新leader
      rf.SwitchRole(ROLE_Follwer)
      rf.currentTerm = reply.Term
      rf.persist()
      return
    }
    // 如果日志同步失败，根据`ConflictIndex`和`ConflictTerm`向前移动找到匹配的日志
    // 是一个term一个term向前移动，具体为：leader中
    // 从后往前找到第一个term为ConflictTerm的日志，如果能够找到这个日志，那就意味着
    // 当前leader和follower都拥有了包括此日志以前的所有日志
    // 所以此时直接从rf.realIndexToLogicIndex(i)+1开始复制日志
    // 如果leader中没有找到term为ConflictTerm的日志，那么就直接发送leader的reply.ConflictIndex所对应的
    // 日志，从而覆盖掉follower中ConflictIndex所对应的日志，因为ConflictIndex为follower中从前往后的第一个term为ConflictTerm的日志，
    // 所以会覆盖掉follower中所有term为ConflictTerm的日志

    // 如果日志同步成功，则增加 nextIndex && matchIndex，并提交复制到超过半数节点的日志。
    // 当前 Leader 只能提交自己任期内的日志，
  }(server) // 匿名函数的输入参数为server

 
rf.SwitchRole(ROLE_Candidate)
  为follower，则votedFor = -1，暂停心跳超时，重置选举超时
  为candidate，则暂停心跳超时
  为leader，则暂停选举超时，重置心跳超时，初始化nextIndex=logsize和matchIndex=-1


rf.StartElection()
  // 由于 sendRpc 会阻塞，所以这里选择新启动 goroutine 去 sendRPC，不阻塞当前协程
  // 开启n-1个协程，n-1为follower的数量，用于给follower发送投票请求
      ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
      reply.Term > rf.currentTerm，则转换为follower，设置新term，return
      投票超过半数，则转换为leader，并发送心跳。
  


AppendEntries()：follower执行。
  // 如果follower的term大，则日志复制失败，并return最新term
  // 如果follower的term小，转换为follower，设置最新term
  // reset选举超时时间
  if lastLogIndex < args.PrevLogIndex {
    reply.ConflictIndex = lastLogIndex + 1
    reply.ConflictTerm = -1
  }

  如果follower的log[PrevLogIndex]的term与leader的不同，则返回此位置的term（reply.ConflictTerm）
  reply.ConflictIndex指向ConflictTerm的第一个日志
  return
  
  找到第一个和leader不匹配的日志下标unmatch_idx，并将匹配的日志复制到follower中。
  根据LeaderCommit和已经复制到的日志，设置commitIndex



RequestVote()：相同任期内如果有多个 candidate，对于 follwer 来说，是先到先服务，后到者理论上无法获取到票，这是通过 votedFor 字段保证的。
  if args.Term == rf.currentTerm && rf.votedFor != -1 && rf.votedFor != args.CandidateId {// 当前节点在currentTerm时，已经给其他candidate投过了票了
    reply.VoteGranted = false
    return
  }
  // 拒绝比较旧的投票
  if (args.LastLogIndex < lastLogIndex && args.LastLogTerm == lastLogTerm) || args.LastLogTerm < lastLogTerm {
    reply.VoteGranted = false
    return
  }
  rf.votedFor = args.CandidateId
  reply.VoteGranted = true
```

领导者选举注意点：
* 1.临界区要尽可能短，像 sleep 或者 sendRPC 都没有必要 lock ，否则可能出现死锁。假设sendRPC和rpc函数需要锁，则有如下问题的例子：A获取锁并向B发送RPC请求，同时B获取锁并向A发送RPC请求，此时A和B的rpc函数都无法获取锁，也就无法处理RPC请求，从而造成了死锁。
* 2.lab 中 sendRpc 是阻塞的，因此尽量使用 goroutine 去执行该操作
* 3.对于角色转换，有一条优先规则是：假设有节点 a 和节点 b 在 rpc 通信，发现节点 a 的 term < 节点 b 的 term，节点 a 无论是什么 role，都需要转换为 follwer。rpc包括投票请求、日志同步、快照同步。
* 4.选举超时有150 ~ 300 ms 的误差，防止多个节点同时成为candidate
* 5.每个follower在同个 term  只有一票
- 6.select接收electionTimer和heartbeatTimer两种信号，follower没有heartbeatTimer、leader没有electionTimer、candidate没有heartbeatTimer，所以必须在切换角色（SwitchRole）时，把角色没有的timer关掉，否则会出现如下问题：select 在多个信号到达时，会随机选择一个，导致会有 timer 超时不触发，比如 election time out 但是没有发起 election。因此切换 role 时需要 stop 没用的timer。- 
- 7.切片是引用类型，所以需要注意可能需要使用copy进行深拷贝。没看懂如下：发送 AppendEntries RPC 的时候，出现了 data race，因为对参数中的日志使用了切片，导致持有了参数的引用，发送 rpc 时需要对参数进行序列化，两者产生 data race。解决方式就是使用如 copy 之类的操作，深拷贝，防止 data race。



日志复制：
- 冲突回退：一个term一个term向前回退日志，如果term内冲突，就一个日志一个日志回退。
- 安全性：拥有更加新日志的节点不会给拥有只旧日志节点投票，从而保证新leader包含所有commit的日志。
- matchIndex[] 表示 follwer 的 commit 情况和 nextIndex[] 表示 follwer 的 log replication 情况
- 日志提交：Figure 8中提到了已经提交日志会被覆盖的情况，为了防止这种情况的出现，需要限制当前 Leader 只能提交自己任期内的日志。从代码上来看，当前 Leader提交了自己任期内的日志以后，此日志以前的所有日志都被提交了。
- 使用rpc发送日志复制或投票请求并收到reply以后，需要重新判断一下`rf.currentTerm != args.Term`，从而保证了leader没有被改变。
- `Logic log`就是假设log中的日志没有删除时的log
`real log`：持久化以后将持久化的log删掉，剩下log就是`real log`，

持久化保存当前日志的状态，状态包括以下内容，如下内容发现变换时就需要执行persist()来持久化状态。持久化的内容：
```go
rf.currentTerm
rf.votedFor
rf.log
rf.snapshot.lastIncludedIndex
rf.snapshot.lastIncludedTerm
```

生成快照：
- 生成快照会截断日志，所以需要记录快照最后一个日志的 Index（lastIncludedIndex） 以及最后一个日志的 Term（lastIncludedTerm），从而计算出真实的log索引。
- 日志压缩，应该需要知道日志的具体内容才能进行日志压缩
- `rf.snapshot.lastIncludedIndex`初始化时不能设置为-1而初始化为0，因为`logicIndexToRealIndex()`中需要通过`logicIndex - rf.snapshot.lastIncludedIndex`计算。如果设置为-1，那么需要考虑的东西就会更多。
```
Snapshot(index int, snapshot []byte)：给index以前的日志生成快照
  rf.log = append([]LogEntry{}, rf.log[realIndex+1:]...)

SendInstallSnapshot(server int)：同步快照给某个节点。
  ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
  更新`nextIndex`和`matchIndex`
  持久化节点状态

InstallSnapshot()：修改快照、commitIndex、lastApplied，rf.applyCh <- ApplyMsg
```

**访问共享变量时，需要加锁。**
- 比如`rf.currentRole`，很多时候执行什么样操作逻辑是基于`rf.currentRole`，那么就要求`rf.currentRole`在操作的过程保持不变。如果中途发生改变了，那么当前执行的逻辑可能就是不对的。所以很多基于currentRole的操作都是需要加锁的。
- 多个线程同时争用的代码块，可能需要加锁。如当前节点的rpc调用，可能被其他多个节点同时调用。




**`rf.applyCh <- ApplyMsg`：在日志提交、快照同步会执行，通知上层进行apply。**
* follower根据leader的`commitIndex`进行提交
* leader在超过半数节点复制到日志以后提交日志
* follower收到快照同步也可能需要`rf.applyCh <- ApplyMsg`





















###  测试代码

[raft测试代码解析](https://zhuanlan.zhihu.com/p/350723333)：创建多个raft节点组成的网络，并将它们连接起来，然后就模拟挂机，并可能向集群发送命令，来测试是否正确选举出leader，是否正确同步日志等。
- labrpc使用channel实现了本地的rpc调用，我们自己定义的raft，也是使用channel来模拟rpc。
- 连接中提到的客户端指的是当前节点要访问其他节点时，需要使用客户端，如raft算法中的`peers     []*labrpc.ClientEnd`

[2A测试详解](https://juejin.cn/post/7141429336120229902)：保证最多只有1或0个leader
* 第一个测试叫initial election，三个raft，检查初始化选举，保证只有一个领导，同时选举成功后一段时间不会触发新选举，总而言之是测试raft稳态
* 第二个测试叫election after network failure，三个raft，会先后分别测试领导挂机、旧领导恢复上线、一个领导和一个马仔挂机、马仔恢复上线、旧领导恢复上线，总而言之是测试回合值更新和判断和选举策略
* 第三个测试叫multiple elections，七个raft，九次迭代，每次迭代都会随机选三个raft挂机，然后再让挂机的raft恢复连接，保证过程中最多一个领导


`TestBasicAgree2B`：`日志是否同步到所有机器中`
- 测试随机关掉或关掉以后再重连follower、leader，查看日志是否可以成功同步。【注】使用数组`cfg.connected`表示raft节点是否可以连接



