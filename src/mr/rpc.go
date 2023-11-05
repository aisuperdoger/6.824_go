package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//
// 通知任务完成
type NotifyArgs struct {
	TaskID   int
	TaskType CoordinatorPhase
	WorkerID int // // 局域网中应该可以使用IP，这里使用的是进程号
}

type NotifyReplyArgs struct {
	Confirm bool
}

// 请求任务
type RequestArgs struct {
}

type ReplyArgs struct {
	FileName  string // map task
	TaskID    int
	TaskType  CoordinatorPhase
	ReduceNum int
	MapNum    int
}

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid()) // os.Getuid()会返回当前进程的实际用户ID（用户id，不是进程id！）
	return s
}
