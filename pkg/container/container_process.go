package container

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

// NewParentProcess 启动一个新进程
/*
	这里是父进程，也就是当前进程执行的内容
	1. 这里的 /proc/self/exe 调用中，/proc/self/ 指的是当前运行进程自己的环境，
		exec 其实就是自己再次调用自身的二进制文件，使用这种方式对床架拿出来的进程进行初始化
	2. 后面的 args 是参数，其中 init 是传给本进程的第一个参数，
		在本例中，其实就是会调用 initCommand 去初始化进程的一些环境和资源
	3. 下面的 clone 参数就是去 fork 出来一个新进程，并且使用 linux namespace 隔离新创建的进程和外部环境
	4. 如果用户指定了 -it 参数，就需要把当前进程的输入输出导入到标准输入输出上
*/
// mydocker run -it /bin/sh  会变为 /proc/self/exe init /bin/sh
// 此处会构建 /proc/self/exe init /bin/sh 这个命令
// 因此执行，相当于再次执行 /proc/self/exe，利用 namespace 构建一个隔离的空间
// init 又会触发 /proc/self/exe 中的  RunContainerInitProcess 逻辑，替换 1 号进程为 /bin/sh
func NewParentProcess(tty bool, command string) *exec.Cmd {
	// 注意此处
	// 会在传入的 command 前新增一个 init 参数，也就是先回调用自身的 init 参数，然后再执行传入的命令 command
	// init 会调用 RunContainerInitProcess 函数
	args := []string{"init", command}
	cmd := exec.Command("/proc/self/exe", args...)
	log.Infof("Run command: %s", cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}
