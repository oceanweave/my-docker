package cmd

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/network"
	"github.com/urfave/cli"
)

var NetworkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{
		// 增加一个 create 子命令,用于需要指定 driver 和 subnet 以及网络名称
		// mydocker network create命令创建一个容器网络,通过 Bridge 网络驱动创建一个名为 testbr 的网络，网段则是 192.168.0.0/24
		// mydocker network create --subset 192.168.0.0/24 --driver bridge testbr0
		{
			Name:  "create",
			Usage: "create a contaienr network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					return fmt.Errorf("missing network name")
				}
				driver := ctx.String("driver")
				subnet := ctx.String("subnet")
				networkName := ctx.Args()[0]

				err := network.CreateNetwork(driver, subnet, networkName)
				if err != nil {
					return fmt.Errorf("create network error: %+v", err)
				}
				return nil
			},
		},
		// 通过 mydocker network list命令显示当前创建了哪些网络
		// 扫描网络配置的目录/var/lib/mydocker/network/network/拿到所有的网络配置信息并打印即可
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(ctx *cli.Context) error {
				network.ListNetwork()
				return nil
			},
		},
		// 使用命令 mydocker network remove命令删除己经创建的网络
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					return fmt.Errorf("missing network name")
				}
				err := network.DeleteNetwork(ctx.Args()[0])
				if err != nil {
					return fmt.Errorf("remove network error: %+v", err)
				}
				return nil
			},
		},
	},
}
