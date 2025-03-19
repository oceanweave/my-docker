package image

import (
	"github.com/oceanweave/my-docker/pkg/constant"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

func CommitContainer(imageName string, containerId string) {
	containerRootPath := GetMergedDir(containerId)
	if _, err := os.Stat(containerRootPath); err != nil {
		log.Errorf("Not find container root path, error: %v", err)
	}

	imageTarPath := constant.GetImageTarDir(containerId)
	imageTar := imageTarPath + imageName + ".tar"
	log.Debugf("CommitContainer-Func Pack the Container[%s] into an ImageTar, Container-Path[%s], ImageTar Save Path[%s]", containerId, containerRootPath, imageTar)
	// -C 表示 切换到这个目录， . 表示当前目录下的所有文件和目录
	// 这样，tar 归档的内容就相对于这个目录，而不会包含 mntPath 这个路径本身
	// CombinedOutput() 会同时捕获 stdout 和 stderr，有助于一次性调试。
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", containerRootPath, ".").CombinedOutput(); err != nil {
		log.Errorf("tar folder %s error %v", containerRootPath, err)
	}
}
