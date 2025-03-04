package constant

import (
	"os"
)

const (
	Perm0777 os.FileMode = 0777
	Perm0755 os.FileMode = 0755
	Perm0644 os.FileMode = 0644
	Perm0622 os.FileMode = 0622
	// 容器打包后镜像的存储路径
	ImageTarPathFormat string = "/var/lib/mydocker/image-tar/%s/"
)
