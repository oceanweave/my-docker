package constant

import "os"

const (
	Perm0777 os.FileMode = 0777
	Perm0755 os.FileMode = 0755
	Perm0644 os.FileMode = 0644
	// 该目录是 overlayfs 的 rootPath，会在此目录创建 upper、work、merged 目录等，lower 目录则是来自于镜像文件目录
	OverlayfsRootURL string = "/go-code/my-docker/"
	// 将 /go-code/my-docker/mnt 改为 /go-code/my-docker/merged 更好理解，其为 overlayfs 的最终目录，也就是容器的 rootfs
	OverlayMergedURL string = "/go-code/my-docker/merged"
)
