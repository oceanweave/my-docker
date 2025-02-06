package cmd

import (
	"fmt"
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
		container.Run(tty, cmd)
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
