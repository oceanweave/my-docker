package subsystems

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/cglimit/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"strconv"
)

type MemorySubSystem struct {
}

// Name 返回 资源Subsystem名字
func (s *MemorySubSystem) Name() string {
	return "memory"
}

// Set 设置 cgroupPath 对应的 cgroup 的内存资源限制
func (s *MemorySubSystem) Set(cgroupPath string, res *types.ResourceConfig) error {
	if res.MemoryLimit == "" {
		return nil
	}
	// 在现有挂载点，找到对应的 memory Subsystem 的 cgroup，将其作为根 cgroup，创建个 子cgroup
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}
	// 设置这个 cgroup 的内存限制，将限制写入到 cgroup 对应目录的 memory.limit_in_bytes 文件中
	memLimitFile := path.Join(subsysCgroupPath, "memory.limit_in_bytes")
	// 若文件不存在，会创建并设置权限 perm；若文件已存在，会写入数据，但不会修改已有的文件权限
	// 覆盖写入 os.WriteFile；追加写入应采用 os.OpenFile 配置 os.O_APPEND|os.O_WRONLY 等，并利用 file.Write 写入
	if err := os.WriteFile(memLimitFile, []byte(res.MemoryLimit), types.Perm0644); err != nil {
		return fmt.Errorf("set cgroup proc fail %v", err)
	}
	return nil
}

// Apply 将 pid 加入到 cgroupPath 对应的 cgroup 中
func (s *MemorySubSystem) Apply(cgroupPath string, pid int) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}
	cgPidFile := path.Join(subsysCgroupPath, "tasks")
	if err := os.WriteFile(cgPidFile, []byte(strconv.Itoa(pid)), types.Perm0644); err != nil {
		return fmt.Errorf("set cgroup proc fail %v", err)
	}
	return nil
}

// Remove 删除 cgroupPath 对应的 cgroup
func (s *MemorySubSystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	log.Infof("Cleaning %s-cgroup-dir [%s]", s.Name(), subsysCgroupPath)
	//err = syscall.Rmdir(subsysCgroupPath)
	// TODO: 目前此命令无法将 cgroup 文件夹清理掉，也并未报错
	err = os.RemoveAll(subsysCgroupPath)
	if err != nil {
		log.Infof("Cleaning %s-cgroup-dir [%s] happen error %s", s.Name(), subsysCgroupPath, err)
	}
	//return os.RemoveAll(subsysCgroupPath)
	return err
}
