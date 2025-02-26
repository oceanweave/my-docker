package image

import (
	"github.com/oceanweave/my-docker/pkg/util"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path"
)

// NewWorkSpace Create an Overlay2 filesystem as container root workspace
/*
	1. 利用镜像包创建只读层
	2. 创建读写层
	3. 对上面两层进行联合挂载，形成 mntURL 挂载点
	- 这里的 rootPath 指的是宿主机上的某个目录，后续将会在该目录创建 overlayfs 的 upper、work、merged 目录
		因此可以将该目录称之为 overlayfs 的 rootPath
*/
func NewWorkSpace(rootPath, mntURL, volume string) {
	createReadOnlyLayer(rootPath)
	createWriteLayer(rootPath)
	createMountPoint(rootPath, mntURL)

	// 如果制定了 volume 则还需要 mount volume
	if volume != "" {
		hostPath, containerPath, err := volumeUrlExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume ")
		}
		mountVolume(mntURL, hostPath, containerPath)
	}
}

// createLower 将busybox作为overlayfs的lower层(也就是容器的只读层）
func createReadOnlyLayer(rootURL string) {
	// 把 busybox 作为 overlayfs 中的 lower 层
	busyboxURL := rootURL + "busybox/"
	busyboxTarURL := rootURL + "busybox.tar"
	// 检查是否已经存在 busybox 文件夹
	exist, err := util.PathExists(busyboxURL)
	if err != nil {
		log.Infof("Fail to judge whether dir %s exists. %v", busyboxURL, err)
	}
	// 不存在则创建目录并将 busybox.tar 解压到 busybox 文件夹中
	if !exist {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			log.Errorf("Mkdir dir %s error. %v", busyboxURL, err)
		}
		if _, err := exec.Command("tar", "-vxf", busyboxTarURL, "-C", busyboxURL).CombinedOutput(); err != nil {
			log.Errorf("Untar dir %s error %v", busyboxTarURL, err)
		}
	}
}

// createDirs 创建 overlayfs 需要的 upper、worker 目录（作为容器的读写层）
// upper 相当于容器读写层，worker 相当于临时存储层
func createWriteLayer(rootURL string) {
	upperURL := rootURL + "upper/"
	if err := os.Mkdir(upperURL, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", upperURL, err)
	}
	workURL := rootURL + "work/"
	if err := os.Mkdir(workURL, 0777); err != nil {
		log.Errorf("mkdir dir %s error. %v", workURL, err)
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

// createMountPoint 挂载overlayfs
func createMountPoint(rootURL string, mntURL string) {
	// mount -t overlay overlay -o lowerdir=lower1:lower2:lower3,upperdir=upper,workdir=work merged
	// workdir 目录是必需的(保留元数据，以及作为缓冲区），它用于 OverlayFS 的各种操作（如 rename、Copy-Up、whiteout 处理），确保文件操作的一致性和原子性
	// 创建对应的挂载目录 mnt ，作为挂载点
	if err := os.Mkdir(mntURL, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", mntURL, err)
	}
	// 把 writeLayer 目录和 busybox 目录 mount 到 mnt 目录下
	// 拼接参数
	// e.g. lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work
	dirs := "lowerdir=" + rootURL + "busybox" + ",upperdir=" + rootURL + "upper" + ",workdir=" + rootURL + "work"
	// dirs := "dirs=" + rootURL + "writeLayer:" + rootURL + "busybox"
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mntURL)
	/*
		报错：[mount -t overlay overlay -o lowerdir=/media/psf/my-docker/busybox,upperdir=/media/psf/my-docker/upper,workdir=/media/psf/my-docker/work /media/psf/my-docker/mnt] ","time":"2025-02-24T16:36:11+08:00"}
				mount: /media/psf/my-docker/mnt: wrong fs type, bad option, bad superblock on overlay, missing codepage or helper program, or other error.
		查看系统日志：dmesg | tail -n 20
				[95251.922075] overlayfs: filesystem on '/media/psf/my-docker/upper' not supported as upperdir
		原因： mac 的 parallel 虚拟机创建的文件系统类型不支持 overlayfs
				df -T /media/psf/my-docker/upper
				Filesystem     Type          1K-blocks      Used  Available Use% Mounted on
				my-docker      fuse.prl_fsd 1948455240 488796116 1459659124  26% /media/psf/my-docker
		解决方法:
			- 经排查发现，该目录是由于 parallel 与 mac 贡献，所以是 fuse.prl_fsd 文件系统类型
			- 但是虚拟上新创建的文件使用的文件系统为 ext4
			- 所以解决方法为：在虚拟机上手动创建目录，将与 mac 共享文件夹的代码，复制到新的目录，即可变为 ext4，从而可以使用 overlayfs
			- 同步可以手动执行命令：rsync -av --delete /media/psf/my-docker/ /go-code/my-docker/
			- rsync -av --delete 源目录 目标目录
		rsync 选项解释：
		-a：保留文件属性（时间戳、权限等）
		-v：显示同步详情
		--delete：如果 /media/psf/my-docker 中的文件被删除，也删除 /go-code/my-docker 里的对应文件
	*/
	log.Debugf("Overlay mount Cmd: %s %s ", cmd.Path, cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
}

// DeleteWorkSpace 清理容器的 overlayfs 相关目录和 volume 相关目录
func DeleteWorkSpace(rootURL string, mntURL string, volume string) {
	log.Debugf("Remove container overlayfs mountPoint and writeLayer.")
	// 如果指定了 volume 则需要 umount volume
	// 重点 Note： 一定要先 umount volume，然后再删除目录，否则由于 bind mount 存在，删除临时目录会导致 volume 目录中的数据丢失
	if volume != "" {
		_, containerPath, err := volumeUrlExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume parameter input is not correct, detail: %v", err)
			return
		}
		umountVolume(mntURL, containerPath)
	}
	DeleteMountPoint(mntURL)
	DeleteWriteLayer(rootURL)
}

func DeleteMountPoint(mntURL string) {
	cmd := exec.Command("umount", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("%v", err)
	}
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove dir %s error %v", mntURL, err)
	}
}

func DeleteWriteLayer(rootURL string) {
	dirs := []string{
		path.Join(rootURL, "merged"),
		path.Join(rootURL, "upper"),
		path.Join(rootURL, "work"),
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			log.Errorf("Remove dir %s error %v", dir, err)
		}
	}
}
