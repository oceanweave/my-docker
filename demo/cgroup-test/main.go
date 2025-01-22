package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
)

/*
总体设计方案:
1. 运行 go run main.go 会执行此程序，创建一个 go 进程 8628
2. 之后 go run 其实分为两步，先编译在运行，因此先编译为 /tmp/main 文件再运行，就是 8667 进程
3. 根据 main 代码，先运行 /proc/self/exe（该指向当前运行文件，也就是 main 文件），也就产生了 8672 进程
4. 根据 main 代码 if 判断，若运行的第一个参数为 /proc/self/exe，就运行 sh 进程，也就是 8678
5. sh 进程会运行 stress 压力测试，也就是 stress 进行 8679， stress 内部会启动自己的进程也就是 8680


           ├─sshd(3501)─┬─sshd(3503)───bash(3643)
           │            ├─sshd(7268)───bash(7560)───go(8628)─┬─main(8667)─┬─exe(8672)─┬─sh(8678)───stress(8679)───stress(8680)
           │            │                                    │            │           ├─{exe}(8674)
           │            │                                    │            │           ├─{exe}(8675)
           │            │                                    │            │           ├─{exe}(8676)
           │            │                                    │            │           └─{exe}(8677)
           │            │                                    │            ├─{main}(8668)
           │            │                                    │            ├─{main}(8669)
           │            │                                    │            ├─{main}(8670)
           │            │                                    │            └─{main}(8671)
           │            │                                    ├─{go}(8629)
           │            │                                    ├─{go}(8630)
           │            │                                    ├─{go}(8631)
           │            │                                    ├─{go}(8632)
           │            │                                    ├─{go}(8651)
           │            │                                    ├─{go}(8652)
           │            │                                    └─{go}(8653)
           │            └─sshd(8681)───bash(8759)───pstree(9067)

root        8628    7560  0 17:15 pts/1    00:00:00 go run main.go
root        8667    8628  0 17:15 pts/1    00:00:00 /tmp/go-build1846075589/b001/exe/main
root        8672    8667  0 17:15 pts/1    00:00:00 /proc/self/exe
root        8678    8672  0 17:15 pts/1    00:00:00 sh -c stress --vm-bytes 400m --vm-keep -m 1
root        8679    8678  0 17:15 pts/1    00:00:00 stress --vm-bytes 400m --vm-keep -m 1
root        8680    8679 60 17:15 pts/1    00:00:12 stress --vm-bytes 400m --vm-keep -m 1
*/

// 挂载了 memory subsystem 的 hierarchy 的根目录位置
// memory subsystem 可以对 memory 进行限制
// hierarchy 是 cgroup 的概念，表示 cgroup 父和子之间的管理和继承关系，简单了解就行
// cgroup 可以管理进程，将 进程id 写入到 cgroup 的 tasks 文件即可
// 不过 cgroup 不具备资源限制能力，所以需要关联到 subsystem，此处就是关联到 memory subsystem，用于后续 memory 限制
// 虽然只是将 cgroup hierarchy 根节点目录，关联到 memory subsystem，但是后续在此目录创建的子 cgroup 都具备 memory 限制能力
// 需要将进程 id 写入到 tasks 文件，将内存限额 写入相应的 memory limit 文件
const (
	cgroupV1MemoryHierarchyMount = "/sys/fs/cgroup/memory"
	cgroupV2MemoryHierarchyMount = "/sys/fs/cgroup"
)

func main() {
	// 2. 在 namespace 隔离的进程空间内，建立个子进程，执行真正的容器任务，stress 进程用于占用内存测试
	//  上面 cgroup 设置了 100m 的 memory 限制，此处申请 200m 内存持续占用，可通过 top 结合 free -m  查看是否完成内存的现值
	if os.Args[0] == "/proc/self/exe" {
		// 容器进程
		fmt.Printf("current pid %d", syscall.Getpid())
		fmt.Println()
		/*
			主机上没有此命令需要预先安装  apt install stress
			这是要执行的具体命令，stress 是一个负载测试工具，用于模拟 CPU、内存、I/O 等资源的压力。
			参数含义：
			--vm-bytes 200m：模拟分配 200 MB 的虚拟内存。
			--vm-keep：保持分配的内存（而不是释放）。
			-m 1：启动 1 个内存压力线程。
		*/
		cmd := exec.Command("sh", "-c", `stress --vm-bytes 400m --vm-keep -m 1`)
		// 设置子进程的系统属性
		// 如果需要配置命名空间或权限隔离等，可以在这里设置相关参数（如 Cloneflags 或 Credential）
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		/*
			绑定子进程的标准输入、标准输出和标准错误到父进程的对应流。
			os.Stdin：允许父进程向子进程提供输入。
			os.Stdout：子进程的输出会直接显示在父进程的控制台上。
			os.Stderr：子进程的错误信息会直接显示在父进程的控制台上。
		*/
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// cmd.Run = cmd.Start + cmd.Wait
		/*
			特性					cmd.Start			cmd.Run
			是否异步				是，立即返回			否，阻塞直到命令完成
			需显式调用cmd.Wait	是					否，自动调用
			适用场景				手动控制子进程生命周期	简单执行并等待命令完成
			子进程状态管理		手动处理（如 Wait）	自动管理子进程生命周期
		*/
		if err := cmd.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	}

	// 1. 相当构建个 namespace 隔离的进程空间（此处设置了 uts、pid、mount ns 隔离），同时创建 cgroup 进行 memory 资源的控制
	//  /proc/self/exe 指代运行当前命令的可执行文件，一般采用 bash 运行该命令，也就是 /proc/self/exe 指代 bash
	/*
		CLONE_NEWIPC	创建一个新的 IPC（进程间通信）命名空间。
		CLONE_NEWNET	创建一个新的网络命名空间。
		CLONE_NEWNS	创建一个新的挂载（Mount）命名空间。
		CLONE_NEWPID	创建一个新的 PID 命名空间，使新进程及其子进程的 PID 从 1 开始。
		CLONE_NEWUSER	创建一个新的用户命名空间，用于隔离用户和权限。
		CLONE_NEWUTS	创建一个新的 UTS（主机名和域名）命名空间。
		CLONE_NEWCGROUP	创建一个新的 CGroup（控制组）命名空间（从 Linux 4.6 开始支持）。
		CLONE_NEWFS	创建一个新的文件系统命名空间（较新的标志，特定场景下使用）。
	*/
	cmd := exec.Command("/proc/self/exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	} else {
		// 得到 fork 出来进程映射在外部命名空间的 pid（就是在宿主机 pid 空间的 id，不是新建 pid ns 中的 pid ）
		fmt.Printf("fork host pid: %v\n", cmd.Process.Pid)
		// 创建 子Cgroup 的目录
		subCgroupDir := "testmemorylimit2"

		if isCgroupV1() {
			os.Mkdir(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir), 0755)
			// 将容器进程加入到这个 Cgroup 中
			ioutil.WriteFile(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir, "tasks"),
				[]byte(strconv.Itoa(cmd.Process.Pid)), 0644)
			// 限制 Cgroup 进程使用 memory 的配额
			ioutil.WriteFile(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir, "memory.limit_in_bytes"),
				[]byte("100m"), 0644)
			// 限制 Cgroup 进程使用 cpu 的配额 下面10000单位是微妙，表示10ms，因此翻译为 10ms 可以使用 5ms，就是 50% cpu 使用率
			// 此处暂未举例，若实现下面命令，cpu.cfs_period_us 表示周期， pu.cfs_quota_us 表示周期内可用时间
			// echo 5000 > /sys/fs/cgroup/cpu/my_cgroup/cpu.cfs_quota_us
			// echo 10000 > /sys/fs/cgroup/cpu/my_cgroup/cpu.cfs_period_us
		} else { // 是 cgroup v1
			os.Mkdir(path.Join(cgroupV2MemoryHierarchyMount, subCgroupDir), 0755)
			// 将容器进程加入到这个 Cgroup 中
			ioutil.WriteFile(path.Join(cgroupV2MemoryHierarchyMount, subCgroupDir, "cgroup.procs"),
				[]byte(strconv.Itoa(cmd.Process.Pid)), 0644)
			// 限制 Cgroup 进程使用 memory 的配额
			ioutil.WriteFile(path.Join(cgroupV2MemoryHierarchyMount, subCgroupDir, "memory.max"),
				[]byte("100m"), 0644)
			// 限制 Cgroup 进程使用 cpu 的配额  10ms 可以使用 5ms，就是 50% cpu 使用率
			ioutil.WriteFile(path.Join(cgroupV2MemoryHierarchyMount, subCgroupDir, "cpu.max"),
				[]byte("5000 10000"), 0644)
		}
		os.Mkdir(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir), 0755)
		// 将容器进程加入到这个 Cgroup 中
		ioutil.WriteFile(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir, "tasks"),
			[]byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		// 限制 Cgroup 进程使用 memory 的配额
		ioutil.WriteFile(path.Join(cgroupV1MemoryHierarchyMount, subCgroupDir, "memory.limit_in_bytes"),
			[]byte("200m"), 0644)
	}
	/*
		cmd.Wait() 会清理资源（释放文件描述符、管道等）
		cmd.Process.Wait() 不会清理资源
		cmd.Process.Wait() 在需要直接操作底层进程或有特殊需求（如仅等待进程退出但不清理资源）时使用，因此适合不清理容器的进程
	*/
	cmd.Process.Wait()

}

/*
	1. 如果系统是 cgroup v2，文件 /sys/fs/cgroup/cgroup.controllers 存在。
	- cat /sys/fs/cgroup/cgroup.controllers
      cpuset cpu io memory hugetlb pids rdma misc

	2. 如果系统是 cgroup v1，该文件不存在，资源限制会分布在不同的子系统目录中（如 cpu, memory 等）。
*/
func isCgroupV1() bool {
	// cgroup v2 的标志文件
	cgroupV2File := "/sys/fs/cgroup/cgroup.controllers"
	var isV1 bool
	if _, err := os.Stat(cgroupV2File); err == nil {
		//  "cgroup v2"
		isV1 = false
	} else if os.IsNotExist(err) {
		//  "cgroup v1"
		isV1 = true
	}
	return isV1
}
