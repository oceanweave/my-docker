package container

import (
	log "github.com/sirupsen/logrus"
	"os"
	"syscall"
)

// RunContainerInitProcess 启动容器的 init 进程
/*
	这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了
	这是本容器执行的第一个进程
	使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况
*/
func RunContainerInitProcess(command string, args []string) error {
	log.Infof("Init command: %s", command)
	// MS_NOEXEC 在本文件系统中不允许运行其他程序
	// MS_NOSUID 在本系统中运行程序的时候， 不允许 set-user-ID 或 set-group-ID
	// MS_NODEV 这个参数是自 从 Linux 2.4 以来 ，所有 mount 的系统都会默认设定的参数。
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// bug修复 —— 修复下面的bug
	// systemd 加入linux之后, mount namespace 就变成 shared by default, 所以你必须显示
	//	声明你要这个新的mount namespace独立。
	syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	// bug: 未加上面的代码，再次执行 my-docker 会提示 宿主机 proc 文件受损，应手动执行 mount -t proc proc /proc 进行修复
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	argv := []string{command}
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
	if err := syscall.Exec(command, argv, os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}
