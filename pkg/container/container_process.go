package container

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/oceanweave/my-docker/pkg/image"
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
func NewParentProcess(tty bool, volume string, containerId string, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
	/* 注意此处
	   - 会在传入的 command 前新增一个 init 参数，也就是先回调用自身的 init 参数，然后再执行传入的命令 command
	   - init 会调用 RunContainerInitProcess 函数
	*/
	// 创建匿名管道用于传递参数，将readPipe作为子进程的ExtraFiles，子进程从readPipe中读取参数
	// 父进程中则通过writePipe将参数写入管道
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	cmd := exec.Command("/proc/self/exe", "init")
	// 通过 namespace 机制，将 init 进程构建为隔离的空间
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	log.Infof("NewParentProcess-Func Buid Init-Command[%s] and Set Init-Namespace-Attr（UTS/PID/NS(MOUNT)/NET/IPC, no set USER)", cmd.String())
	// 若 mydocker run 配置 -it 参数，会开启此部分，用于将容器进程的输入输出展示到终端上
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 对于后台运行容器，将 stdout、stderr 重定向到日志文件中，便于后续查看
		dirPath := fmt.Sprintf(ContainerInfoPathFormat, containerId)
		if err := os.MkdirAll(dirPath, constant.Perm0622); err != nil {
			log.Errorf("NewParentProcess mkdir %s error %v", dirPath, err)
			return nil, nil
		}
		stdLogFilePath := dirPath + LogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParentProcess create file %s error %v", stdLogFile, err)
			return nil, nil
		}
		log.Debugf("Detach Runing Container[%s] Save Stdout and Stderr to LogFile[%s]", containerId, stdLogFile)
		cmd.Stdout = stdLogFile
		cmd.Stderr = stdLogFile
	}
	// 默认 0 标准输入  1 标准输出  2 标准错误
	// 因此此处 3——匿名管道
	cmd.ExtraFiles = []*os.File{readPipe}
	log.Infof("NewParentProcess-Func Set Anonymous-Pipe to Init-Command")

	// 为容器创建 overlayfs 目录，挂载镜像文件等，同时创建目录，将 -v 参数指定的宿主机目录挂载到容器内
	image.NewWorkSpace(imageName, containerId, volume)
	// mydocker init 会通过 pwd 获取该路径，通过 privotRoot 将容器进程的根目录改为当前目录
	log.Debugf("NewParentProcess-Func Set Overlayfs-Merged-Path[%s] for Init-Command(will use for PivotRootfs later)", image.GetMergedDir(containerId))
	cmd.Dir = image.GetMergedDir(containerId)
	// 获取当前宿主机的环境变量，配置到容器中
	cmd.Env = append(os.Environ(), envSlice...)
	log.Infof("NewParentProcess-Func Set Envs[%s] to Init-Command(if have setting-flag -e)", envSlice)

	return cmd, writePipe
}
