package container

import (
	"github.com/oceanweave/my-docker/pkg/cglimit"
	"github.com/oceanweave/my-docker/pkg/image"
	log "github.com/sirupsen/logrus"
)

func CleanStoppedContainerResource(containerId string) {
	// 1. 根据容器 ID 查询容器信息
	containerInfo, err := getInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	volume := containerInfo.Volume

	// 2. 清理容器的残留资源
	log.Infof("Staring Resource-Cleanning ...")
	// 2.1 清理 cgroup 目录
	cgroupManager := cglimit.NewCgroupManager("mydocker-cgroup")
	cgroupManager.Destroy()
	// 2.2 清理 volume 目录和 overlayfs 文件目录
	// 现 umount volume 目录，避免删除 overlayfs 目录时将宿主机文件删除
	image.DeleteWorkSpace(containerId, volume)
	// 2.3 清理宿主机文件记录的 Container 信息
	DeleteContainerInfo(containerId)
	log.Infof("Finsh Container Resource Clean.")
}
