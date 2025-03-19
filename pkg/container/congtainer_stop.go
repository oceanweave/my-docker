package container

import (
	"encoding/json"
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"strconv"
	"syscall"
)

func StopContainer(containerId string) {
	// 1. 根据容器 ID 查询容器信息
	containerInfo, err := getInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	pidInt, err := strconv.Atoi(containerInfo.Pid)
	if err != nil {
		log.Errorf("Convert pid from string to int error %v", err)
		return
	}
	log.Infof("1. StopContainer-Func get ContainerInfo ContainerID[%s]-PID[%d]", containerId, pidInt)
	// 2. 发送 SIGTERM 信号
	if err = syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		log.Errorf("Stop contaienr %s error %v", containerId, err)
		return
	}
	log.Infof("2. StopContainer-Func send [syscall.SIGTERM] to kill Container-Process-PID[%d]", pidInt)
	// 3. 修改容器信息，将容器置为 STOP 状态，并清空 PID
	containerInfo.Status = STOP
	containerInfo.Pid = " "
	newContentBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Json marshal %s error %v", containerId, err)
		return
	}
	// 4. 重新写回存储容器信息的文件
	dirPath := fmt.Sprintf(ContainerInfoPathFormat, containerId)
	configFilePath := path.Join(dirPath, ConfigName)
	if err := os.WriteFile(configFilePath, newContentBytes, constant.Perm0622); err != nil {
		log.Errorf("Write file %s error: %v", configFilePath, err)
	}
	log.Infof("3. StopContainer-Func change Container-Status to [STOP] and Remov [PID] info, Save the ContainerInfo to [%s]", configFilePath)
	log.Infof("4. StopContainer-Func Finsh Stop Container-[%s]", containerId)
}
