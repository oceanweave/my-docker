package container

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"syscall"
)

/*
	pivotroot 和 chroot 有什么区别？
    - pivot_root 是把整个系统切换到一个新的 root 目录，会移除对之前 root 文件系统的依赖，这样你就能够 umount 原先的 root 文件系统。
    - 而 chroot 是针对某个进程，系统的其他部分依旧运行于老的 root 目录中。
	int pivot_root(const char *new_root, const char *put_old);
	- new_root：新的根文件系统的路径。
	- put_old：将原根文件系统移到的目录。
	- 使用 pivot_root 系统调用后，原先的根文件系统会被移到 put_old 指定的目录，而新的根文件系统会变为 new_root 指定的目录。这样，当前进程就可以在新的根文件系统中执行操作。
	- 注意：new_root 和 put_old 不能同时存在当前 root 的同一个文件系统中。

*/

func setUpMount() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)

	// systemd 加入linux之后, mount namespace 就变成 shared by default, 所以你必须显示
	// 声明你要这个新的mount namespace独立。
	// 如果不先做 private mount，会导致挂载事件外泄，后续执行 pivotRoot 会出现 invalid argument 错误
	err = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	err = privotRoot(pwd)
	if err != nil {
		log.Errorf("privotRoot failed, detail: %v", err)
		return
	}

	// mount /proc
	/*
		syscall.MS_NOEXEC
		禁止在 /proc 目录下执行二进制文件。
		防止 /proc 下的可执行文件被直接运行，提高安全性。

		syscall.MS_NOSUID
		在 /proc 目录下，setuid 和 setgid 位不会生效。
		防止具有 setuid 或 setgid 位的二进制文件在 /proc 目录下运行时提升权限。

		syscall.MS_NODEV
		禁止在 /proc 目录中访问设备文件（/dev 设备节点）。 —— 即使有设备文件的路径，也不会被识别为设备文件
		由于 /proc 主要用于进程信息和内核接口，防止 /proc 下的设备文件被误用或被攻击者利用。
	*/
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// "tmpfs 是基于 文件系统，使用 RAM 和 swap 分区来存储数据"。确实如此，tmpfs 是一种 内存文件系统，它将数据存储在 RAM（物理内存） 和 swap（交换分区） 之中，而不是物理磁盘。
	// 由于前面 pivotRoot 切换了 rootfs，因此这里重新 mount 一下 /dev 目录
	// 不挂载 /dev，会导致容器内部无法访问和使用许多设备，这可能导致系统无法正常工作
	// 挂载 tmpfs 到 /dev 可减少对宿主机的依赖，提高隔离性
	// syscall.MS_STRICTATIME 是一个挂载标志（mount flag），用于控制 文件访问时间（atime） 的更新策略。它的作用是 强制每次访问文件时都更新 atime（访问时间），即使 relatime 机制默认会优化掉某些更新
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

func privotRoot(newRoot string) error {
	/**
	  NOTE：PivotRoot调用有限制，newRoot和oldRoot不能在同一个文件系统下。
	  因此，为了使当前root的老root和新root不在同一个文件系统下，这里把root重新mount了一次。
	  bind mount是把相同的内容换了一个挂载点的挂载方法

		2. bind 挂载（MS_BIND）
		MS_BIND：表示使用 绑定挂载。它将 源目录 和 目标目录 中的文件系统连接起来。MS_BIND 挂载不会复制数据，而是使得两个目录共享相同的文件系统内容。
		例如，syscall.Mount("/mnt/source", "/mnt/target", "bind", 0, "") 会使 /mnt/source 和 /mnt/target 指向相同的数据，任何在一个目录上的变更都会反映到另一个目录。
		3. MS_REC
		MS_REC：表示递归挂载。它会递归地影响到挂载点下的所有子挂载点。
		如果你只使用 MS_BIND，它只会对 当前目录 执行挂载操作。而使用 MS_REC 后，它会递归地将 该目录下的所有子目录 也挂载到目标位置。
		例如，MS_REC 会将 /root 目录下的所有子目录（如 /root/subdir）也进行绑定挂载。

	  解决 pivot_root 问题：通过将当前根文件系统重新挂载到自己，确保新旧根文件系统不在同一个文件系统下，从而满足 pivot_root 的要求。
		解释：
		- pivot_root 的限制：pivot_root 系统调用有一个重要的限制：新的根文件系统（newRoot）和旧的根文件系统（oldRoot） 必须不在同一个文件系统 上。这是因为 pivot_root 会把当前的根文件系统切换到新的根文件系统，它需要保证旧的根文件系统是在一个独立的文件系统中。
	    - 挂载根文件系统到自身：为了绕过这个限制，代码通过 bind mount 将当前的根文件系统（root）挂载到它自身。这种做法实际上是将根文件系统重新挂载到一个新的挂载点（虽然文件系统的内容没有变化），但它让根文件系统看起来像是在一个不同的挂载点上。这样，pivot_root 就可以成功执行，因为它认为新的根和旧的根是在不同的文件系统上。
			- 为什么使用 MS_BIND：MS_BIND 是一个挂载标志，表示创建一个绑定挂载，这样挂载点的内容和原始文件系统是一样的，但挂载点本身是独立的。
	        - MS_REC 使得这个挂载递归到所有子目录，确保所有的子目录也都能挂载到新的根上。
	    - 总结来说，这段代码的作用是通过将根文件系统挂载到自身来确保 pivot_root 能顺利执行，因为它需要确保旧根和新根不在同一文件系统上。
	*/
	// newRoot 是 oldRoot 上的一个目录
	// 此处执行 bind-mount，相当于为 newRoot 创建一个挂载点，表示和 oldRoot 是不同的系统，用于后续 pivotRoot 更改 root 目录
	if err := syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return errors.Wrap(err, "mount rootfs to itself")
	}
	// 此目录放置 oldRoot
	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}
	// 执行 pivot_root 系统调用，将系统 rootfs oldRoot 切换到新的 rootfs newRoot
	// PivotRoot 调用会把 old_root 挂载到 pivotDir，也就是 newRoot/.pivot_root，挂载点现在依然可以在 mount 命令中看到
	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return errors.WithMessagef(err, "pivotRoot failed, new_root: %v old_put: %v", newRoot, pivotDir)
	}
	// 现在 newRoot 作为了 / 根目录，因此修改当前的工作目录到根目录
	if err := syscall.Chdir("/"); err != nil {
		return errors.WithMessage(err, "chdir to / failed")
	}

	// 最后再把 old_root umount，即 umount rootfs/.pivot_root
	// 由于当前已经是在 rootfs 下了，就不能在用上面的 rootfs/.pivot_root 这个路径了，现在直接用 /.pivot_root 这个路径即可
	pivotDir = filepath.Join("/", ".pivot_root")
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return errors.WithMessage(err, "umount pivot_root dir")
	}
	// 删除临时文件夹（也就是旧的 root 目录）
	return os.Remove(pivotDir)

}
