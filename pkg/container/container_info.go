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
	"text/tabwriter"
	"time"
)

const (
	RUNNING                 = "running"
	STOP                    = "stopped"
	EXIT                    = "exited"
	ContainersInfoPath      = "/var/lib/mydocker/containers/"
	ContainerInfoPathFormat = ContainersInfoPath + "%s/"
	ConfigName              = "config.json"
	IDLength                = 10
	LogFile                 = "%s-json.log"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	CreatedTime string `json:"createdTime"`
	Status      string `json:"status"`
	Volume      string `json:"volume"`
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

func RecordContainerInfo(containerPID int, cmdArray []string, containerName, containerId string, volume string) error {
	// 如果未指定容器名，则使用随机生成的 containerID
	if containerName == "" {
		containerName = containerId
	}
	command := strings.Join(cmdArray, "")
	containerInfo := &ContainerInfo{
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
	dirPath := fmt.Sprintf(ContainerInfoPathFormat, containerId)
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
	dirPath := fmt.Sprintf(ContainerInfoPathFormat, containerID)
	if err := os.RemoveAll(dirPath); err != nil {
		log.Errorf("Remove dir %s error %v", dirPath, err)
	}
}

func ListContainers() {
	// 读取存放容器信息目录下的所有文件
	files, err := os.ReadDir(ContainersInfoPath)
	if err != nil {
		log.Errorf("read dir %s error %v", ContainersInfoPath, err)
		return
	}
	containersInfo := make([]*ContainerInfo, 0, len(files))
	for _, file := range files {
		tmpContainerInfo, err := getContainerInfo(file)
		if err != nil {
			log.Errorf("get container info error %v", err)
			continue
		}
		containersInfo = append(containersInfo, tmpContainerInfo)
	}
	// 使用 tabwriter.NewWriter 在控制台打印出容器信息
	// tabwriter 是引用的 text/tabwriter 类库，用于在控制台打印对齐的表格
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	_, err = fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")
	if err != nil {
		log.Errorf("Fprint error %v", err)
	}
	for _, item := range containersInfo {
		_, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Id,
			item.Name,
			item.Pid,
			item.Status,
			item.Command,
			item.CreatedTime,
			item.Volume)
		if err != nil {
			log.Errorf("Fprint error %v", err)
		}
	}
	if err = w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
	}
}

// getContainerInfo 从宿主机上容器 config.json 文件中解析出容器信息
func getContainerInfo(file os.DirEntry) (*ContainerInfo, error) {
	// 根据文件名拼接出完整路径
	configFileDir := fmt.Sprintf(ContainerInfoPathFormat, file.Name())
	configFileDir = path.Join(configFileDir, ConfigName)
	// 读取容器配置文件
	content, err := os.ReadFile(configFileDir)
	if err != nil {
		log.Errorf("read file %s error %v", configFileDir, err)
		return nil, err
	}
	info := new(ContainerInfo)
	if err = json.Unmarshal(content, info); err != nil {
		log.Errorf("json unmarshal error %v", err)
		return nil, err
	}
	return info, nil
}

func getInfoByContainerId(containerId string) (*ContainerInfo, error) {
	dirPath := fmt.Sprintf(ContainerInfoPathFormat, containerId)
	configFilePath := path.Join(dirPath, ConfigName)
	log.Debugf("Container json file path: %v", configFilePath)
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "read file %s", configFilePath)
	}
	/*
		此种写法错误：
		- 声明 var containerInfo *ContainerInfo 时，它是 nil 指针。
		- json.Unmarshal(nil, nil) 无法解析 JSON，会报错 "Unmarshal(nil *ContainerInfo)"。
		- 修复方法：初始化 ContainerInfo 结构体，确保 Unmarshal 能正确解析
	*/
	//var containerInfo *ContainerInfo
	//if err = json.Unmarshal(contentBytes, containerInfo); err != nil {
	//	return nil, err
	//}
	var containerInfo ContainerInfo
	if err = json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return nil, err
	}
	return &containerInfo, nil
}
