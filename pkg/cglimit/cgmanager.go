package cglimit

import (
	"github.com/oceanweave/my-docker/pkg/cglimit/subsystems"
	"github.com/oceanweave/my-docker/pkg/cglimit/types"
	"github.com/sirupsen/logrus"
)

type CgroupManager struct {
	// cgroup 在 hierarchy 中的路径，相当于创建的 cgroup 目录相对于 root cgroup 目录的路径
	Path string
	// 资源配置
	Resource *types.ResourceConfig
}

// 此处的 path 就是在各个资源 subsystem cgroup 下新创建的 cgroup 名称
func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// Apply 将进程 pid 加入到这个 cgroup 中
func (c *CgroupManager) Apply(pid int) error {
	// 调用各个资源 subsystem 的真正方法，可以理解为 批量操作，将 pid 添加到所有资源subsystem 的 task 文件中
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Apply(c.Path, pid)
		if err != nil {
			logrus.Errorf("apply subsystem: %s, err: %s", subSysIns.Name(), err)
		}
	}
	return nil
}

// Set 设置 cgroup 资源限制
func (c *CgroupManager) Set(res *types.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Set(c.Path, res)
		if err != nil {
			logrus.Errorf("set subsystem: %s, err: %s", subSysIns.Name(), err)
		}
	}
	return nil
}

// Destroy 释放 cgroup
func (c *CgroupManager) Destroy() error {
	logrus.Infof("Cleaning %s subsystem-cgroup-dirs", c.Path)
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Remove(c.Path)
		if err != nil {
			logrus.Errorf("set subsystem: %s, err: %s", subSysIns.Name(), err)
		}
	}
	logrus.Infof("Finsh clean %s subsystem-cgroup-dirs", c.Path)
	return nil
}
