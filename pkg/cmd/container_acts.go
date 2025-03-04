package cmd

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/container"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

var ListCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the contaienrs",
	Action: func(ctx *cli.Context) error {
		container.ListContainers()
		return nil
	},
}

var StopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a contaienr, e.g. mydocker stop contaienr-id(1234567890)",
	Action: func(ctx *cli.Context) error {
		// 期望输入是： mydocker stop 容器Id，若没有指定参数直接打印错误
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("missing container id")
		}
		containerId := ctx.Args().Get(0)
		// 从宿主机记录文件查询容器 pid，将其 kill，之后设置状态为 Stopped
		container.StopContainer(containerId)
		return nil
	},
}

var RemoveCommand = cli.Command{
	Name:  "rm",
	Usage: "remove unused container（if running, add '-f' flag can force delete）, e.g. mydocker stop contaienr-id)",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "f", // 强制删除
			Usage: "force delete running container",
		},
	},
	Action: func(ctx *cli.Context) error {
		// 期望输入是： mydocker stop 容器Id，若没有指定参数直接打印错误
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("missing container id")
		}
		containerId := ctx.Args().Get(0)
		force := ctx.Bool("f")
		// 清理容器进程 stop 后残留的 cgroup、overlayfs、volume 等目录
		// ./mydocker rm containerId 用于清理 stopped 状态的容器残留目录
		// ./mydocker rm -f containerId 用于清理 running 状态的容器残留目录
		container.RemoveContainer(containerId, force)
		return nil
	},
}

var LogCommannd = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("please input your container name")
		}
		containerId := ctx.Args().Get(0)
		container.LogContainer(containerId)
		return nil
	},
}

var ExecCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command into container",
	Action: func(ctx *cli.Context) error {
		// 如果环境变量存在，说明 C 代码已经运行过了，即 setns 系统调用已经执行了，这里就直接返回，避免重复执行
		if os.Getenv(container.EnvExecPid) != "" {
			log.Infof("pid callback pid %v", os.Getpid())
			return nil
		}
		// 格式: mydocker exec 容器名称 命令， 因此至少会有两个参数
		if len(ctx.Args()) < 2 {
			return fmt.Errorf("missing container name or command")
		}
		containerId := ctx.Args().Get(0)
		// 将除了容器名之外的参数作为命令部分
		var commandArray []string
		for _, arg := range ctx.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		container.ExecContainer(containerId, commandArray)
		return nil
	},
}
