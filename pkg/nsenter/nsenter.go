package nsenter

/*
// setns 是 GNU 扩展的系统调用，需要启用 _GNU_SOURCE
#define _GNU_SOURCE
#include <unistd.h>
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>


// 这个 constructor 关键字表示 enter_namespace 会在程序 main() 运行之前执行
__attribute__((constructor)) void enter_namespace(void) {
	// 这里的代码会在 Go 运行时启动前执行，他会在单线程的 C 上下文中运行
	char *mydocker_pid;
	// 从环境变量 mydocker_pid 读取目标容器的 PID
	mydocker_pid = getenv("mydocker_pid");
	if (mydocker_pid) {
		fprintf(stdout, "got mydocker_pid=%s\n", mydocker_pid);
	} else {
		fprintf(stdout, "missing mydocker_pid env skip nsenter");
		// 如果没有执行 PID 就不需要继续执行，直接退出
		return;
	}
	// 从环境变量 mydocker_cmd 读取 exec 要在容器内执行的命令
	char *mydocker_cmd;
	mydocker_cmd = getenv("mydocker_cmd");
	if (mydocker_cmd) {
		fprintf(stdout, "got mydocker_cmd=%s\n", mydocker_cmd);
	} else {
		fprintf(stdout, "missing mydocker_cmd env skip nsenter");
		// 如果没有执行 PID 就不需要继续执行，直接退出
		return;
	}


	int i;
	char nspath[1024];
	// 需要进入的 5 中 namespace
	char *namespaces[] = {"ipc", "uts", "net", "pid", "mnt"};

	for (i=0;i<5;i++) {
		// 拼接对应路径，类似于 /proc/pid/ns/ipc 这样
		sprintf(nspath, "/proc/%s/ns/%s", mydocker_pid, namespaces[i]);
		int fd = open(nspath, O_RDONLY);
		// 执行 setns 系统调用，进入对应 namespace
		// setns(fd, 0) 中的 0 表示"不指定 Namespace 类型"，直接进入 fd 指向的 Namespace
		// setns(fd, 0) 让当前进程进入 fd 指向的 Namespace
		if (setns(fd,0) == -1) {
			fprintf(stderr, "setns on %s namespace failed: %s\n", namespaces[i], strerror(errno));
		} else {
			fprintf(stdout, "setns on %s namespace succeeded\n", namespaces[i]);
		}
		close(fd);
	}
	// 在进入的 Namespce 中执行指定命令，然后退出
	int res = system(mydocker_cmd);
	exit(0);
	return;
}

*/
import "C"
