package cmd

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/image"
	"github.com/urfave/cli"
)

var CommitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit container to image,e.g. mydocker commit 123456789 myimage",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 2 {
			return fmt.Errorf("missing container name and image name")
		}
		containerId := context.Args().Get(0)
		imageName := context.Args().Get(1)
		image.CommitContainer(imageName, containerId)
		return nil
	},
}
