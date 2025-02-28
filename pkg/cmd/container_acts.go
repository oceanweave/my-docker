package cmd

import (
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
