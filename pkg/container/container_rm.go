package container

import log "github.com/sirupsen/logrus"

func RemoveContainer(containerId string, force bool) {
	containerInfo, err := getInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	switch containerInfo.Status {
	case STOP:
		CleanStoppedContainerResource(containerId)
	case RUNNING:
		if !force {
			log.Errorf("Couldn't remove running container [%s], Stop the container before attempting removal or"+
				" force remove", containerId)
			return
		}
		StopContainer(containerId)
		RemoveContainer(containerId, force)
	default:
		log.Errorf("Couldn't remove container, invalid status %s", containerInfo.Status)
		return
	}
}
