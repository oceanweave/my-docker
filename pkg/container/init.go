package container

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const fdIndex = 3

// RunContainerInitProcess 启动容器的 init 进程
/*
	这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了
	这是本容器执行的第一个进程
	使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况
*/
func RunContainerInitProcess(command string, args []string) error {
	log.Infof("Init command: %s", command)
	// 此处涵盖了 mountProc 所以将其进行了注释
	setUpMount()
	//mountProc()
	// 从 pipe 中读取命令
	cmdArray := readUserCommand()
	if len(cmdArray) == 0 {
		return errors.New("run container get user command error, cmdArray is nil")
	}
	// 从 Path 中查找命令，这样用户可以将 /bin/sh 简写为 sh, 此处会为 sh 搜索到	全路径为 /usr/bin/sh
	cmdPath, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find path %s", cmdPath)
	/*
		- 本函数最后的syscall.Exec是最为重要的一句黑魔法，正是这个系统调用实现了完成初始化动作并将用户进程运行起来的操作。
		- 首先，使用 Docker 创建起来一个容器之后，会发现容器内的第一个程序，也就是 PID 为 1 的那个进程，是指定的前台进程。
			但是，我们知道容器创建之后，执行的第一个进程并不是用户的进程，而是 init 初始化的进程。
			这时候，如果通过 ps 命令查看就会发现，容器内第一个进程变成了自己的 init,这和预想的是不一样的。
		- 有没有什么办法把自己的进程变成 PID 为 1 的进程呢？
		- 这里 execve 系统调用就是用来做这件事情的。
		- syscall.Exec 这个方法，其实最终调用了 Kernel 的 int execve(const char *filename, char *const argv[], char *const envp[);这个系统函数。
			它的作用是执行当前 filename 对应的程序,它会覆盖当前进程的镜像、数据和堆栈等信息，包括 PID，这些都会被将要运行的进程覆盖掉。
		- 也就是说，调用这个方法，将用户指定的进程运行起来，把最初的 init 进程给替换掉，这样当进入到容器内部的时候，就会发现容器内的第一个程序就是我们指定的进程了。
	*/
	if err := syscall.Exec(cmdPath, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf("RunContainerInitProcess exec:" + err.Error())
	}
	return nil
}

// 为 init 进程重新挂载 proc 文件系统，展示该进程内的所有进程信息（不暴露宿主机上进程）
func mountProc() {
	// MS_NOEXEC 在本文件系统中不允许运行其他程序
	// MS_NOSUID 在本系统中运行程序的时候， 不允许 set-user-ID 或 set-group-ID
	// MS_NODEV 这个参数是自 从 Linux 2.4 以来 ，所有 mount 的系统都会默认设定的参数。
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// bug修复 —— 修复下面的bug
	// systemd 加入linux之后, mount namespace 就变成 shared by default, 所以你必须
	//	显示声明你要这个新的mount namespace独立。
	syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// bug: 未加上面的代码，再次执行 my-docker 会提示 宿主机 proc 文件受损，应手动执行 mount -t proc proc /proc 进行修复
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
}

// 用于读取父进程通过匿名通道传过来的用户参数
func readUserCommand() []string {
	// uintptr(3 ）就是指 index 为3的文件描述符，也就是传递进来的管道的另一端，至于为什么是3，具体解释如下：
	/*	因为每个进程默认都会有3个文件描述符，分别是标准输入、标准输出、标准错误。这3个是子进程一创建的时候就会默认带着的，
		前面通过ExtraFiles方式带过来的 readPipe 理所当然地就成为了第4个。
		在进程中可以通过index方式读取对应的文件，比如
		index0：标准输入
		index1：标准输出
		index2：标准错误
		index3：带过来的第一个FD，也就是readPipe
		由于可以带多个FD过来，所以这里的3就不是固定的了。
		比如像这样：cmd.ExtraFiles = []*os.File{a,b,c,readPipe} 这里带了4个文件过来，分别的index就是3,4,5,6
		那么我们的 readPipe 就是 index6,读取时就要像这样：pipe := os.NewFile(uintptr(6), "pipe")
	*/
	pipe := os.NewFile(uintptr(fdIndex), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
