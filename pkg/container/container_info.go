package container

import (
	"encoding/json"
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	RUNNING        = "running"
	STOP           = "stopped"
	EXIT           = "exited"
	InfoPath       = "/var/lib/mydocker/containers/"
	InfoPathFormat = InfoPath + "%s/"
	ConfigName     = "config.json"
	IDLength       = 10
	LogFile        = "%s-json.log"
)

type Info struct {
	Pid         string `json:"pid"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	CreatedTime string `json:"createdTime"`
	Status      string `json:"status"`
}

func GenerateContainerID() string {
	return randStringsBytes(IDLength)
}

func randStringsBytes(n int) string {
	letterBytes := "1234567890"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	// 随机生成一个 letterBytes 长度内的数字，获取 letterBytes 对应的元素，将其组合成指定的 n 长度，作为 containerID
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func RecordContainerInfo(containerPID int, cmdArray []string, containerName, containerId string) error {
	// 如果未指定容器名，则使用随机生成的 containerID
	if containerName == "" {
		containerName = containerId
	}
	command := strings.Join(cmdArray, "")
	containerInfo := &Info{
		Id:          containerId,
		Pid:         strconv.Itoa(containerPID),
		Command:     command,
		CreatedTime: time.Now().Format("2006-01-02 15:04:05"),
		Status:      RUNNING,
		Name:        containerName,
	}

	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		return errors.WithMessage(err, "container info marshal failed")
	}
	jsonStr := string(jsonBytes)
	// 持久化存储到宿主机上
	// 拼接出存储容器信息文件的路径，如果目录不存在则级联创建
	dirPath := fmt.Sprintf(InfoPathFormat, containerId)
	if err := os.MkdirAll(dirPath, constant.Perm0622); err != nil {
		return errors.WithMessagef(err, "mkdir %s failed", dirPath)
	}
	// 将容器信息写入文件 /var/lib/mydocker/containers/${continerId}/config.json
	fileName := path.Join(dirPath, ConfigName)
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		return errors.WithMessagef(err, "create file %s failed", fileName)
	}
	log.Debugf("ContainerInfo save path: %s", fileName)
	if _, err = file.WriteString(jsonStr); err != nil {
		return errors.WithMessagef(err, "write container info to file %s failed", fileName)
	}
	return nil
}

func DeleteContainerInfo(containerID string) {
	dirPath := fmt.Sprintf(InfoPathFormat, containerID)
	if err := os.RemoveAll(dirPath); err != nil {
		log.Errorf("Remove dir %s error %v", dirPath, err)
	}
}
