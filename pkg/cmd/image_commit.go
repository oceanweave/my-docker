package cmd

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/oceanweave/my-docker/pkg/image"
	"github.com/urfave/cli"
)

var CommitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit container to image",
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			return fmt.Errorf("missing image name")
		}
		imageName := ctx.Args().Get(0)
		image.CommitContainer(imageName, constant.OverlayMergedURL)
		return nil
	},
}
