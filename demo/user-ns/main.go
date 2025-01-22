package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("/bin/sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
	}
	// 报错: fork/exec /bin/sh: operation not permitted
	// 这行代码不能用的原因是linux kernel 3.19对user namespace做出了修改，所以原来的方式不行了
	//cmd.SysProcAttr.Credential = &syscall.Credential{
	//	Uid: uint32(1),
	//	Gid: uint32(1),
	//}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(-1)
}
