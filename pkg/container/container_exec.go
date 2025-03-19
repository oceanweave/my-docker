package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

const (
	EnvExecPid = "mydocker_pid"
	EnvExecCmd = "mydocker_cmd"
)

func ExecContainer(containerId string, cmdArray []string) {
	// 根据传进来的容器名获取对应的 PID
	pid, err := GetPidByContainerId(containerId)
	if err != nil {
		log.Errorf("Exec container GetContainerPidByContainerID %s error %v", containerId, err)
		return
	}

	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 把命令拼接成字符串，便于传递
	cmdStr := strings.Join(cmdArray, " ")
	_ = os.Setenv(EnvExecPid, pid)
	_ = os.Setenv(EnvExecCmd, cmdStr)
	// 由于环境变量是继承自父进程的，因此这个 exec 进程的环境变量其实是继承自宿主机的，
	// 所以在 exec 进程内看到的环境变量其实是宿主机的环境变量。
	// 此处根据进程 pid 获取进程的环境变量
	containerEnvs := getEnvsByPid(pid)
	cmd.Env = append(os.Environ(), containerEnvs...)
	log.Infof("Container Exec-Operation will Run Command[%s], And Set Envs[%s/%s]，So the import-init-nsenter-cgo will set your-Process[%s] into Container[%s]-Process's Namespace", cmdStr, EnvExecPid, EnvExecCmd, pid, containerId)

	if err = cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerId, err)
	}
}

// getEnvsByPid 根据指定的 PID 来获取对应进程的环境变量。
// 由于进程存放环境变量的位置是/proc/<PID>/environ，因此根据给定的 PID 去读取这个文件，便可以获取环境变量。
// 在文件的内容中，每个环境变量之间是通过\u0000分割的，因此以此为标记来获取环境变量数组。
func getEnvsByPid(pid string) []string {
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("Read file %s error %v", path, err)
		return nil
	}
	// env split by \u000
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs
}
