package container

import (
	"github.com/oceanweave/my-docker/pkg/cglimit"
	resource "github.com/oceanweave/my-docker/pkg/cglimit/types"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

// 进程关系  run --> init --> 容器进程（用户参数）
// run（my-docker run -it 用户参数，用户参数匿名管道发送端; 为此进程配置 cgroup 限制本身及所有衍生子进程的资源）
// --> init（隐式逻辑，/proc/self/exe init 等同于 my-docker init，用户参数匿名管道接收端，利用用户参数启动容器进程)
// --> 容器进程（用户输入的参数）
func Run(tty bool, cmdArray []string, res *resource.ResourceConfig) {
	// 构建出 init 命令，将匿名管道 read 端放到 init 命令中，同时为 init 命令配置 namespace 和重定向等参数
	// parent 就是 init 命令，也就是容器父进程；run 进程是 init 的父进程
	// 此处返回的匿名管道 write 部分，是为了 run 进程将用户参数传递给 init 进程，从而启动真正的容器进程
	//（采用管道传输用户参数，是为了避免用户传输的参数过长）
	parent, writePipe := NewParentProcess(tty)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	// 执行 init 命令，也就是准备创建容器进程
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	// 创建 cgroup manager, 并通过调用 ser 和 apply 设置资源限制并限制在容器上生效
	// TODO: 此处 mydocker-cgroup 为创建的资源限制子 cgroup，若启动多个进程应该名称设置为不同（否则会引发bug，某个进程结束会删除该目录），所以此处待改进
	cgroupManager := cglimit.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	_ = cgroupManager.Set(res)
	_ = cgroupManager.Apply(parent.Process.Pid)
	// 在 init 子进程创建后，run 进程通过管道发送用户参数给 init 子进程
	sendInitCommand(cmdArray, writePipe)
	_ = parent.Wait()
	log.Infof("container process stoped ！！！")
	//os.Exit(-1)
}

func sendInitCommand(cmdArray []string, writePipe *os.File) {
	command := strings.Join(cmdArray, " ")
	log.Infof("command all is %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
