package constant

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

func GetImageTarDir(containerId string) string {
	imageTarPath := fmt.Sprintf(ImageTarPathFormat, containerId)
	/*
		若目录不存在就创建，存在就不创建
		- os.Mkdir 仅创建指定目录，如果父级目录不存在，则报错
		- os.MkdirAll 递归创建所有缺失的父级目录，如果目录已存在，不会报错。
	*/
	if err := os.MkdirAll(imageTarPath, 0777); err != nil {
		log.Errorf("Mkdir dir %s error. %v", imageTarPath, err)
	}
	return imageTarPath
}
