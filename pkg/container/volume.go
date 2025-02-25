package container

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path"
	"strings"
)

// volumeUrlExtract 通过冒号分割解析 volume 参数，比如 -v /tmp:/tmp
func volumeUrlExtract(volume string) (sourcePath, destinationPath string, err error) {
	parts := strings.Split(volume, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid volume [%s], must split by `:` ", volume)
	}
	sourcePath, destinationPath = parts[0], parts[1]
	if sourcePath == "" || destinationPath == "" {
		return "", "", fmt.Errorf("invalid volume [%s], path can't be empty", volume)
	}
	return sourcePath, destinationPath, nil
}

/*
挂载数据卷的过程如下。
1）首先，创建宿主机文件目录
2）然后，拼接出容器目录在宿主机上的真正目录，格式为：$mntPath/$containerPath
因为之前使用了 pivotRoot 将$mntPath 作为容器 rootfs，因此这里的容器目录也可以按层级拼接最终找到在宿主机上的位置。
3）最后，执行 bind mount 操作，至此对数据卷的处理也就完成了。
*/
// mountVolume 使用 bind mount 挂载 volume
// 使用 bind mount 而不是用 软/硬链接原因，容器和宿主机属于不同的文件系统（容器使用 overlay 联合挂载文件系统，同时还具备 mount namespace 与宿主机隔离）
func mountVolume(mntPath, hostPath, containerPath string) {
	// 创建宿主机目录
	if err := os.Mkdir(hostPath, constant.Perm0777); err != nil {
		log.Infof("mkdir parent dir %s error. %v", hostPath, err)
	}
	// 拼接出对应的容器目录在宿主机上的位置（也就是在 overlay 系统中的目录），并创建对应的目录
	// mntPath 就是容器的 rootfs（即 overlayfs 的最终目录）
	containerPathInHost := path.Join(mntPath, containerPath)
	if err := os.Mkdir(containerPathInHost, constant.Perm0777); err != nil {
		log.Infof("mkdir container dir %s error. %v", containerPathInHost)
	}
	// 通过 bind mount 将宿主机目录挂载到容器目录
	//  mount -o bind /hostPath /containerPath
	cmd := exec.Command("mount", "-o", "bind", hostPath, containerPathInHost)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("mount volume failed. %v", err)
	}
}

func umountVolume(mntPath, containerPath string) {
	// mntPath 为容器在宿主机上的挂载点，例如 /root/merged
	// containerPath 为 volume 在容器中对应的目录，例如 /root/tmp
	// containerPathInHost 则是容器中目录在宿主机上的具体位置，例如 /root/merged/root/tmp
	containerPathInHost := path.Join(mntPath, containerPath)
	cmd := exec.Command("umount", containerPathInHost)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		log.Errorf("Umount volume failed. %v", err)
	}
}
