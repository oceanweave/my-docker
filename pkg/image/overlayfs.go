package image

import (
	"github.com/oceanweave/my-docker/pkg/util"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

// NewWorkSpace Create an Overlay2 filesystem as container root workspace
/*
1. createLowerLayer 利用镜像创建只读层 lower
2. createUpperLayer 创建读写层，包括 upper、work、merged 目录
3. 将 lower、upper、work 通过 overlayfs 联合挂载到 merged 目录，作为容器根目录
4. 若配置 volume，在 merged 目录中创建对应目录，将宿主机目录挂载到该目录上
*/
func NewWorkSpace(imageName, containerId, volume string) {
	createLowerLayer(imageName, containerId)
	createUpperLayer(containerId)
	mountOverlayFS(containerId)

	// 如果制定了 volume 则还需要 mount volume
	if volume != "" {
		hostPath, containerPath, err := volumeUrlExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume ")
		}
		mntPath := GetMergedDir(containerId)
		mountVolume(mntPath, hostPath, containerPath)
	}
}

// createLower 将busybox作为overlayfs的lower层(也就是容器的只读层）
// 将镜像解压到 lower 目录
func createLowerLayer(imageName string, containerId string) {
	// 把 busybox 作为 overlayfs 中的 lower 层
	imageTarPath := GetImageDir(imageName)
	lowerPath := GetLowerDir(containerId)
	// 判断镜像 tar 包是否存在
	_, err := os.Stat(imageTarPath)
	if err != nil {
		// Fatal 相当于执行了 log.Infof 和 os.Exit(1)，打印后直接退出程序，避免程序继续执行
		log.Fatalf("Couldn't find image %s, Error: %s", imageTarPath, err)
	}
	// lowerPath 目录不存在也不报错，后续进行创建，其他错误进行报错
	exist, err := util.PathExists(lowerPath)
	if err != nil {
		log.Errorf("Fail to judge whether dir %s exists. %v", lowerPath, err)
	}

	// lowerPath 不存在就创建
	if !exist {
		// 使用 MkdirAll，父级目录不存在会进行创建；使用 Mkdir 则无法创建父级目录，会报错目录不存在
		if err := os.MkdirAll(lowerPath, 0777); err != nil {
			log.Fatalf("Mkdir dir %s error. %v", lowerPath, err)
		}
	}

	// 解压镜像 tar 包到 lowerPath
	if _, err := exec.Command("tar", "-vxf", imageTarPath, "-C", lowerPath).CombinedOutput(); err != nil {
		log.Fatalf("Untar dir %s error %v", imageTarPath, err)
	}
}

// createDirs 创建 overlayfs 需要的 upper、work 目录（作为容器的读写层）
// upper 相当于容器读写层，work 相当于临时存储层
func createUpperLayer(containerId string) {
	dirs := []string{
		GetMergedDir(containerId),
		GetUpperDir(containerId),
		GetWorkDir(containerId),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0777); err != nil {
			log.Fatalf("mkdir dir %s error. %v", dir, err)
		}
	}
}

/*
sudo mount -t overlay overlay -o lowerdir=/lower,upperdir=/upper,workdir=/work /merged
- lowerdir=/lower: 只读层。
- upperdir=/upper: 可写层。
- workdir=/work: 工作目录。
- /merged: 最终的联合视图。
为什么 work 文件夹是必需的
- 技术实现需求: OverlayFS 需要在文件操作过程中维护中间状态，而 work 文件夹提供了这个功能。
- 避免冲突: 如果没有 work 文件夹，多个并发操作可能会导致文件系统状态不一致。
- 性能优化: work 文件夹帮助 OverlayFS 高效地处理文件操作，减少对 upperdir 和 lowerdir 的直接访问。
*/
// mountOverlayFS 挂载overlayfs
func mountOverlayFS(containerId string) {
	// 拼接参数
	// e.g. lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work
	dirs := GetOverlayFSDirs(GetLowerDir(containerId), GetUpperDir(containerId), GetWorkDir(containerId))
	mergedPath := GetMergedDir(containerId)
	//完整命令：mount -t overlay overlay -o lowerdir=/root/{containerID}/lower,upperdir=/root/{containerID}/upper,workdir=/root/{containerID}/work /root/{containerID}/merged
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mergedPath)
	log.Infof("mount overlayfs: [%s]", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("%v", err)
	}
}

// DeleteWorkSpace Delete the UFS filesystem while container exit
/*
和创建顺序相反
1. 若 volume 不为空，先 umount 其在 merged 目录中的路径，避免先删除 merged 目录导致宿主机文件丢失
2. umount overlayfs 挂载点，就是 umount merged 目录
3. 删除 overlayfs 相关目录（upper、lower、work、merged、containerId 目录）
*/
func DeleteWorkSpace(containerId string, volume string) {
	log.Debugf("Remove container overlayfs mountPoint and writeLayer.")
	// 如果指定了 volume 则需要 umount volume
	// 重点 Note： 一定要先 umount volume，然后再删除目录，否则由于 bind mount 存在，删除临时目录会导致 volume 目录中的数据丢失
	if volume != "" {
		_, containerPath, err := volumeUrlExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume parameter input is not correct, detail: %v", err)
			return
		}
		mntPath := GetMergedDir(containerId)
		umountVolume(mntPath, containerPath)
	}
	umountOverlayFS(containerId)
	DeleteDirs(containerId)
}

func umountOverlayFS(containerId string) {
	mntPath := GetMergedDir(containerId)
	cmd := exec.Command("umount", mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Infof("umountOverlayFS,cmd:%v", cmd.String())
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

func DeleteDirs(containerId string) {
	dirs := []string{
		GetMergedDir(containerId),
		GetUpperDir(containerId),
		GetWorkDir(containerId),
		GetLowerDir(containerId),
		GetRootDir(containerId), // 当前容器 root 目录也要删除
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			log.Errorf("Remove dir %s error %v", dir, err)
		}
	}
}
