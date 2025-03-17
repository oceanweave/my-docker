package container

import (
	"github.com/oceanweave/my-docker/pkg/image"
	log "github.com/sirupsen/logrus"
)

func RemoveContainer(containerId string, force bool) {
	containerInfo, err := getInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	switch containerInfo.Status {
	// 容器状态为 STOP，开始清理资源
	case STOP:
		CleanStoppedContainerResource(containerId)
	// 容器状态为 RUNNING，若采用 force 删除，先停止容器，再清理资源；不是 force 删除，就什么都不做
	case RUNNING:
		if !force {
			log.Errorf("Couldn't remove running container [%s], Stop the container before attempting removal or"+
				" force remove", containerId)
			return
		}
		// 此处设计很有趣
		// 先停止容器，再此调用此函数进行判断
		StopContainer(containerId)
		RemoveContainer(containerId, force)
	default:
		log.Errorf("Couldn't remove container, invalid status %s", containerInfo.Status)
		return
	}
}

func CleanStoppedContainerResource(containerId string) {
	// 1. 根据容器 ID 查询容器信息
	containerInfo, err := getInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	volume := containerInfo.Volume

	// 2. 清理容器的残留资源
	log.Infof("Staring Container[%s] Resource-Cleanning ...", containerInfo.Id)
	// 2.1 清理 cgroup 目录
	cgroupManager := containerInfo.CgroupManager
	cgroupManager.Destroy()
	// 2.2 清理 volume 目录和 overlayfs 文件目录
	// 现 umount volume 目录，避免删除 overlayfs 目录时将宿主机文件删除
	image.DeleteWorkSpace(containerId, volume)
	// 2.3 清理宿主机文件记录的 Container 信息
	DeleteContainerInfo(containerId)
	// 2.4 释放容器 IP
	if containerInfo.IP != "" {
		ReleaseContainerIP(containerInfo)
	}
	log.Infof("Finsh Container[%s] Resource Clean.", containerInfo.Id)
}
