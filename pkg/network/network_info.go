package network

import (
	"encoding/json"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

var (
	// 默认将容器网络信息存储在/var/lib/mydocker/network/network/目录
	defaultNetworkPath = "/var/lib/mydocker/network/network"
	drivers            = map[string]Driver{}
)

func init() {
	// 加载网络驱动
	var bridgeDriver = BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver

	// 文件不存在则创建
	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("check %s exist failed, detail: %v", defaultNetworkPath, err)
			return
		}
		if err = os.MkdirAll(defaultNetworkPath, constant.Perm0644); err != nil {
			log.Errorf("create %s failed, detail: %v", defaultNetworkPath, err)
			return
		}
	}
}

// 将当前网络信息持久化到宿主机上指定文件 dumpPath
func (net *Network) dump(dumpPath string) error {
	// 检查保存的目录是否存在，不存在则创建
	if _, err := os.Stat(dumpPath); err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("check %s exist failed, detail: %v", dumpPath, err)
			return err
		}
		if err = os.MkdirAll(dumpPath, constant.Perm0644); err != nil {
			return errors.Wrapf(err, "create network dump path %s failed", dumpPath)
		}
	}
	// 保存的文件是网络的名字
	netPath := path.Join(dumpPath, net.Name)
	// 打开保存的文件用于写入,后面打开的模式参数分别是存在内容则清空、只写入、不存在则创建
	netFile, err := os.OpenFile(netPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, constant.Perm0644)
	if err != nil {
		return errors.Wrapf(err, "open file %s failed", dumpPath)
	}
	defer netFile.Close()

	// 将 Network 信息转为 json，存入到文件中
	netJson, err := json.Marshal(net)
	if err != nil {
		return errors.Wrapf(err, "Marshal %v failed", net)
	}

	_, err = netFile.Write(netJson)
	log.Debugf("Dump Network-%s-Info to File: %s", net.Name, netPath)
	return errors.Wrapf(err, "write %s failed", netJson)

}

// 加载指定的单个文件
// 从指定位置 dumpPath 加载网络信息，并反序列化为 Network
func (net *Network) load(dumpFilePath string) error {
	// 打开配置文件
	netConfigFile, err := os.Open(dumpFilePath)
	if err != nil {
		return err
	}
	defer netConfigFile.Close()
	// 从配置文件中读取网络配置的 json 字符串
	netJson := make([]byte, 2000)
	n, err := netConfigFile.Read(netJson)
	if err != nil {
		return err
	}

	err = json.Unmarshal(netJson[:n], net)
	return errors.Wrapf(err, "unmarshal %s failed", netJson[:n])
}

// 加载指定目录下的多个文件
// 读取 defaultNetworkPath 目录下的 Network 信息存放到内存中，便于使用
func LoadNetwork() (map[string]*Network, error) {
	networks := map[string]*Network{}

	// 检查网络配置目录中的所有文件，并执行第二个参数中的函数指针去处理目录下的所有文件
	err := filepath.Walk(defaultNetworkPath, func(netPath string, info fs.FileInfo, err error) error {
		// 如果是目录则跳过
		if info.IsDir() {
			return nil
		}
		// 加载文件名作为网络名
		// 第一个参数为 目录路径，忽略；第二个参数为需要的文件名
		_, netName := path.Split(netPath)
		net := &Network{
			Name: netName,
		}
		log.Debugf("Try load Network-%s-ConfigFile: %s", net.Name, netPath)
		// 调用前面介绍的 load 方法加载网络的配置信息
		if err = net.load(netPath); err != nil {
			log.Errorf("error load network: %s", err)
		}
		// 将网络的配置信息加入到 networks 字典中
		networks[netName] = net
		return nil
	})
	return networks, err
}

func (net *Network) remove(dumpPath string) error {
	// 检查网络对应的配置文件状态，如果文件已经不存在就直接返回
	fullPath := path.Join(dumpPath, net.Name)
	if _, err := os.Stat(fullPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	// 否则删除这个网络对应的配置文件
	return os.Remove(fullPath)
}
