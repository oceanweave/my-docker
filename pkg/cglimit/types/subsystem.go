package types

/*
	核心思想
	- 核心思想就是根据传递过来的参数，创建对应的 cgroups 并配置 subsystem
	- 例如指定了 -m 100m 就创建 memory subsystem，限制只能使用 100m 内存
*/

// Subsystem 接口，每个Subsystem可以实现下面的4个接口，
// 抽象接口，每种资源 都需要实现该 Subsystem 定义的四种接口
// 这里将cgroup抽象成了path,原因是cgroup在hierarchy的路径，便是虚拟文件系统中的虚拟路径
type Subsystem interface {
	// Name 返回当前 subsystem 的名称，比如 cpu、memory
	Name() string
	// Set 设置某个 cgroup 在这个 subsystem 中的资源限制
	// 例如 memory subsystem 则需将配置写入 memory.limit_in_bytes 文件
	//		cpu subsystem 则是写入 cpu.cfs_period_us 和 cpu.cfs_quota_us
	Set(path string, res *ResourceConfig) error
	// Apply 将进程添加到某个进程中
	Apply(path string, pid int) error
	// Remove 移除某个 cgroup
	Remove(path string) error
}
