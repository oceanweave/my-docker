package container

import (
	"github.com/oceanweave/my-docker/pkg/cglimit"
	resource "github.com/oceanweave/my-docker/pkg/cglimit/types"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/oceanweave/my-docker/pkg/image"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
)

// 进程关系  run --> init --> 容器进程（用户参数）
// run（my-docker run -it 用户参数，用户参数匿名管道发送端; 为此进程配置 cgroup 限制本身及所有衍生子进程的资源）
// --> init（隐式逻辑，/proc/self/exe init 等同于 my-docker init，用户参数匿名管道接收端，利用用户参数启动容器进程)
// --> 容器进程（用户输入的参数）
func Run(tty bool, cmdArray []string, res *resource.ResourceConfig, volume string, containerName string) {
	// 0. 创建容器 id
	containerId := GenerateContainerID()
	log.Debugf("Current containerID is [%s]", containerId)

	// 1. 构建 init 命令，得到匿名管道写入端
	// 构建出 init 命令，将匿名管道 read 端放到 init 命令中，同时为 init 命令配置 namespace 和重定向等参数
	// parent 就是 init 命令，也就是容器父进程；run 进程是 init 的父进程
	// 此处返回的匿名管道 write 部分，是为了 run 进程将用户参数传递给 init 进程，从而启动真正的容器进程
	//（采用管道传输用户参数，是为了避免用户传输的参数过长）
	parent, writePipe := NewParentProcess(tty, volume, containerId)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	// 执行 init 命令，init 命令会创建容器进程，此处是实现容器进程的运行
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	// 持久化容器信息到宿主机上指定的 json 文件中
	err := RecordContainerInfo(parent.Process.Pid, cmdArray, containerName, containerId, volume)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	// 2. 根据资源配置，创建 cgroup 目录并设置对应的配额限制
	// 创建 cgroup manager, 并通过调用 set 和 apply 设置资源限制并限制在容器上生效
	// TODO: 此处 mydocker-cgroup 为创建的资源限制子 cgroup，若启动多个进程应该名称设置为不同（否则会引发bug，某个进程结束会删除该目录），所以此处待改进
	// 经过测试，此 TODO 居然 没问题，可能是因为 cgroup 机制原因，同时开启多个 mydocker 运行容器，结束一个容器进行，并没有清理此目录，可能是因为检测到有进程占用
	cgroupManager := cglimit.NewCgroupManager("mydocker-cgroup")
	// 只有配置资源情况下，才创建 cgroup 目录；若没有配置资源，就不创建
	if cglimit.IsSetResource(res) {
		log.Debugf("Create Cgroup Dir.")
		// 此处会在配置的 resource 对应的 Default cgroup 下创建 mydocker-cgroup 目录
		_ = cgroupManager.Set(res)
		// TODO: 若容器没有配置任何资源限额，此处会提示找不到 /sys/fs/cgroup/${resource}/mydocker-cgroup/ 目录，因为没配置资源限制，上面 Set 就不会创建此目录；不过影响不大，就是个报错
		_ = cgroupManager.Apply(parent.Process.Pid)
	}

	// 3. 将用户参数发送给 init 进程，从而生成容器进程（此处用户参数的进程会替换 init，作为 1 号进程）
	// 在 init 子进程创建后，run 进程通过管道发送用户参数给 init 子进程
	sendInitCommand(cmdArray, writePipe)

	// 4. 等待容器进程结束
	// 4.1 若开启 tty，则前台等待容器进程结束，未结束会阻塞
	if tty {
		// 等待进程结束，并释放相关资源（如 stdin, stdout, stderr）; 只能调用一次，否则会报错
		_ = parent.Wait()
		// 仅等待进程结束，不会清理 stdin/stdout/stderr
		//_, _ = parent.Process.Wait()
		// 容器进程结束后，进行相应的资源清理
		log.Infof("Container process stoped ！！！Staring Resource-Cleanning ...")
		cgroupManager.Destroy()
		rootURL := constant.OverlayfsRootURL
		mntURL := constant.OverlayMergedURL
		image.DeleteWorkSpace(rootURL, mntURL, volume)
		DeleteContainerInfo(containerId)
		log.Infof("Finsh Container Resource Clean.")
	}
	// 4.2 容器进程 detach，若没开启 tty，my-docker 主进程会结束，容器进程会被【宿主机上的1号进程】纳管（在宿主机上执行 ps -ef 或 pstree -pl 可查看到）
	// - 由于主进程已经结束，因此当容器进程结束后，相应的 overlay 目录等需要手动清理
	// - cd ${constant.OverlayfsRootURL}
	// - umount merged && rm -rf merged/ upper/ work/
	// TODO: -d (detach) 情况下，完成容器资源的清理工作

	//os.Exit(-1)
}

// 将用户参数写入匿名管道，发送给 init 进程，从而初始化成容器进程
func sendInitCommand(cmdArray []string, writePipe *os.File) {
	command := strings.Join(cmdArray, " ")
	log.Infof("command all is %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
