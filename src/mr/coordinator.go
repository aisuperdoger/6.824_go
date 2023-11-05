package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type CoordinatorPhase int32

const (
	PHASE_MAP      CoordinatorPhase = 1
	PHASE_REDUCE   CoordinatorPhase = 2
	PHASE_FINISH   CoordinatorPhase = 3
	PHASE_WAITTING CoordinatorPhase = 4
)

type TaskStatus int32

const (
	INIT    TaskStatus = 0
	RUNNING TaskStatus = 1
	DONE    TaskStatus = 2
)

type MRTask struct {
	fileName  string
	taskID    int
	startTime int64
	status    TaskStatus
	taskType  CoordinatorPhase
}

type Coordinator struct {
	// Your definitions here.
	phase        CoordinatorPhase
	MRTasks      []MRTask
	lock         sync.Mutex
	RunningTasks chan MRTask
	nReduce      int
	nMap         int
}

// 设置 task 启动相关信息
func (c *Coordinator) setupTaskById(taskID int) {
	for idx := range c.MRTasks {
		task := &c.MRTasks[idx]
		if task.taskID == taskID {
			task.startTime = time.Now().Unix()
			// break // 这里应该加一个break吧
		}
		
	}
}

func (c *Coordinator) AllTaskDone() bool {
	taskAllDone := true
	for _, task := range c.MRTasks {
		if task.status != DONE {
			fmt.Printf("Task %v not finish ...", task)
			taskAllDone = false
			break
		}
	}
	return taskAllDone
}

// 阶段流转
func (c *Coordinator) TransitPhase() {
	// 生成对应阶段 task
	// 如果多个线程同时进入下面代码中，那么可能会出现一个线程执行了PHASE_MAP->PHASE_REDUCE，
	// 然后一个线程执行了PHASE_REDUCE->PHASE_FINISH。这就导致状态从PHASE_MAP，直接跳到PHASE_FINISH
	c.lock.Lock() 
	newPhase := c.phase
	switch c.phase {
	case PHASE_MAP:
		fmt.Printf("TransitPhase: PHASE_MAP -> PHASE_REDUCE\n")
		newPhase = PHASE_REDUCE
		c.MRTasks = []MRTask{}                          // 清空 map task
		c.RunningTasks = make(chan MRTask, c.nReduce+1) // resize
		for i := 0; i < c.nReduce; i++ {
			task := MRTask{
				taskID:   i, // task id
				status:   INIT,
				taskType: PHASE_REDUCE,
			}
			c.MRTasks = append(c.MRTasks, task)
			fmt.Printf("[PHASE_REDUCE]Add Task %v\n", task)
			c.RunningTasks <- task
		}
	case PHASE_REDUCE:
		fmt.Printf("TransitPhase: PHASE_REDUCE -> PHASE_FINISH\n")
		newPhase = PHASE_FINISH
	}
	c.phase = newPhase
	c.lock.Unlock()
}

// 任务超时检查
func (c *Coordinator) CheckTimeoutTask() bool {
	/*
		1. 如果没有超时，则直接 return，等待任务完成 or 超时
		2. 有超时，则直接分配该任务给 worker
	*/
	TaskTimeout := false
	now := time.Now().Unix()
	for _, task := range c.MRTasks {
		if (now-task.startTime) > 10 && task.status != DONE {
			fmt.Printf("now=%d,task.startTime=%d\n", now, task.startTime)
			c.RunningTasks <- task
			TaskTimeout = true
		}
	}
	return TaskTimeout
}

// Your code here -- RPC handlers for the worker to call.
// worker 申请 task
func (c *Coordinator) RequestTask(args *RequestArgs, reply *ReplyArgs) error {
	if len(c.RunningTasks) == 0 {
		fmt.Printf("not running task ...\n")
		// 先检查是否所有任务都已完成
		if c.AllTaskDone() {
			fmt.Printf("All Task Done ... \n")
			c.TransitPhase() // 任务结束，则切换状态
		} else if !c.CheckTimeoutTask() { // 有超时任务，则将任务放入c.RunningTasks并跳出if。【问题】放入c.RunningTasks以后，此任务完成了怎么办？
			// 没有任务超时，则返回PHASE_WAITTING, 让 worker 等待所有任务完成
			fmt.Printf("waiting task finish ... \n")
			reply.TaskType = PHASE_WAITTING
			return nil
		}
	}

	if c.phase == PHASE_FINISH {
		fmt.Printf("all mr task finish ... close coordinator\n")
		reply.TaskType = PHASE_FINISH
		return nil
	}

	task, ok := <-c.RunningTasks
	if !ok {
		fmt.Printf("task queue empty ...\n")
		return nil
	}

	c.lock.Lock() // 应该可以不用加锁
	defer c.lock.Unlock()
	c.setupTaskById(task.taskID) // 设置任务的开始时间
	reply.FileName = task.fileName
	reply.TaskID = task.taskID
	reply.TaskType = c.phase
	reply.ReduceNum = c.nReduce
	reply.MapNum = c.nMap
	//fmt.Printf("Task Apply %v\n", reply)

	return nil
}

func (c *Coordinator) CommitTask(args *NotifyArgs) {
	switch c.phase {
	case PHASE_MAP:
		fmt.Printf("[PHASE_MAP] Commit Task %v\n", args)
		 // 每个任务中的文件内容都被解析处理到了nReduce个文件中。 args.TaskID, i分别相当于mapId, reduceId
		 // 为什么要重命名，而不是直接在worker中直接将名字命名为finalMapOutFile(args.TaskID, i)。
		 // 答：worker在完成任务以后，会通知coordinator，coordinator可能要进行一些信息的校验，从而再进行提交。
		 // （虽然本代码没有进行任校验，但是为了完整性，我们将代码这么写）
		for i := 0; i < c.nReduce; i++ { 
			err := os.Rename(tmpMapOutFile(args.WorkerID, args.TaskID, i),
				finalMapOutFile(args.TaskID, i))
			if err != nil {
				fmt.Printf("os.Rename failed ... err=%v\n", err)
				return
			}
		}

	case PHASE_REDUCE:
		fmt.Printf("[PHASE_REDUCE] Commit Task %v\n", args)
		err := os.Rename(tmpReduceOutFile(args.WorkerID, args.TaskID),
			finalReduceOutFile(args.TaskID))
		if err != nil {
			fmt.Printf("os.Rename failed ... err=%v\n", err)
			return
		}
	}
}

func (c *Coordinator) RequestTaskDone(args *NotifyArgs, reply *NotifyReplyArgs) error {
	now := time.Now().Unix()
	for idx := range c.MRTasks {
		task := &c.MRTasks[idx]
		if task.taskID == args.TaskID {
			// fix：这里应该判断一下是否超时了，如果超时，应该放回到c.RunningTasks中
			// 我添加的代码 开始
			if (now-task.startTime) > 10 && task.status != DONE {
				fmt.Printf("now=%d,task.startTime=%d\n", now, task.startTime)
				c.RunningTasks <- *task
				reply.Confirm =false
				return nil
			}
			// 我添加的代码 结束

			task.status = DONE
			c.CommitTask(args)
			break
		}
	}
	return nil

}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c) // c中的公开方法可以被远程调用
	// 将RPC服务注册到默认的HTTP服务器中
	// 当有RPC请求到达HTTP服务器时，它将被传递给RPC处理程序进行处理
	rpc.HandleHTTP() 
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname) // 删除对应文件
	// // Unix域套接字 网络地址是通过文件系统中一个不存在的路径来表示的
	//  Unix域套接字  数据传输在内核中完成，不需经过网络设备，所以更加高效
	l, e := net.Listen("unix", sockname) 
	if e != nil {
		log.Fatal("listen error:", e)
	}
	fmt.Println("starting ...")
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	c.lock.Lock()
	ret := c.phase == PHASE_FINISH
	c.lock.Unlock()
	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
// 调试时，pg-*.txt不会自动解析成多个文件， 所有调试时files为["pg-*.txt"]
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.nReduce = nReduce
	// Your code here.
	c.phase = PHASE_MAP
	c.RunningTasks = make(chan MRTask, len(files)+1) //缓存大小为len(files)+1，只有缓存满了，send才会阻塞 Channel可以作为一个先入先出(FIFO)的队列
	c.nMap = len(files)
	fmt.Printf("start make coordinator ... file count=%d\n", len(files))
	for index, fileName := range files {
		task := MRTask{
			fileName: fileName, // task file
			taskID:   index,    // task id
			status:   INIT,
			taskType: PHASE_MAP,
		}
		c.MRTasks = append(c.MRTasks, task)
		fmt.Printf("[PHASE_MAP]Add Task %v %v\n", fileName, index)
		c.RunningTasks <- task
	}

	c.server()
	return &c
}
	