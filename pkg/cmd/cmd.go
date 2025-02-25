package cmd

import (
	"fmt"
	resource "github.com/oceanweave/my-docker/pkg/cglimit/types"
	"github.com/oceanweave/my-docker/pkg/container"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// RunCommand 首字母要大写，小写表示私有（别的包无法使用）
var RunCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -it [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", // 简单起见，这里吧 -i 和 -t 参数合并成一个
			Usage: "enable tty",
		},
		cli.StringFlag{
			Name:  "mem", //限制进程内存使用量，为了避免和 stress 命令的 -m 桉树冲突，这里使用 -mem
			Usage: "memory limie, e.g.: -mem 100m",
		},
		cli.StringFlag{
			Name:  "cpu",
			Usage: "cpu quota, e.g.: -cpu 100", // 限制进程 cpu 使用率
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit, e.g.: -cpuset 2,4", // 应该是绑核，将其绑定到哪个核上
		},
		cli.StringFlag{ // 数据卷
			Name:  "v",
			Usage: "volume, e.g.: -v /etc/conf:/etc/conf",
		},
	},
	/*
		这里是 run 命令执行的真正函数
		1. 判断参数是否包含 command
		2. 获取用户指定的 command
		3. 调用 Run function 去准备启动容器
	*/
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("Missing container command")
		}
		cmd := ctx.Args()
		log.Infof("Run comand args[0]: %s", cmd)
		// 更具 -it flag 判断是否需要开启  输入输出重定向到终端
		tty := ctx.Bool("it")
		resConf := &resource.ResourceConfig{
			MemoryLimit: ctx.String("mem"),
			CpuSet:      ctx.String("cpuset"),
			CpuCfsQuota: ctx.Int("cpu"),
		}
		log.Info("resConf:", resConf)
		volume := ctx.String("v")
		container.Run(tty, cmd, resConf, volume)
		return nil
	},
}

var InitCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in contaienr. Do not call it outside",
	/*
		1. 获取传递过来的 command 参数
		2. 执行容器初始化操作
	*/
	Action: func(ctx *cli.Context) error {
		log.Infof("init come on")
		cmd := ctx.Args().Get(0)
		log.Infof("command: %s", cmd)
		err := container.RunContainerInitProcess(cmd, nil)
		return err
	},
}
