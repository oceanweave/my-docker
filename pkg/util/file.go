package util

import "os"

// PathExists 忽略路径不存在错误，可以由用户自行创建；其他错误进行报错
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
