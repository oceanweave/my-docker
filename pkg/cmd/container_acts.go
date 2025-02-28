package cmd

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/container"
	"github.com/urfave/cli"
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
	Usage: "remove a contaienr（first stop，then can remove）, e.g. mydocker stop contaienr-id，then，mydocker rm contaienr-id(1234567890)",
	Action: func(ctx *cli.Context) error {
		// 期望输入是： mydocker stop 容器Id，若没有指定参数直接打印错误
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("missing container id")
		}
		containerId := ctx.Args().Get(0)
		// 清理容器进程 stop 后残留的 cgroup、overlayfs、volume 等目录
		container.CleanStoppedContainerResource(containerId)
		return nil
	},
}
