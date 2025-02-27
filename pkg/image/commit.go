package image

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

func CommitContainer(imageName string, mntPath string) {
	if _, err := os.Stat(mntPath); err != nil {
		log.Errorf("Not find image-path, error: %v", err)
	}
	/*
		若目录不存在就创建，存在就不创建
		- os.Mkdir 仅创建指定目录，如果父级目录不存在，则报错
		- os.MkdirAll 递归创建所有缺失的父级目录，如果目录已存在，不会报错。
	*/
	if err := os.MkdirAll(constant.ImageTarPath, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", constant.ImageTarPath, err)
	}
	imageTar := constant.ImageTarPath + imageName + ".tar"
	fmt.Println("commitContainer imageTar:", imageTar)
	// -C 表示 切换到这个目录， . 表示当前目录下的所有文件和目录
	// 这样，tar 归档的内容就相对于这个目录，而不会包含 mntPath 这个路径本身
	// CombinedOutput() 会同时捕获 stdout 和 stderr，有助于一次性调试。
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntPath, ".").CombinedOutput(); err != nil {
		log.Errorf("tar folder %s error %v", mntPath, err)
	}
}
