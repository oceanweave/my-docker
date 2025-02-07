package types

// ResourceConfig 用于传递资源限制配置的结构体，包含内存限制、cpu 时间片权重，cpu 核心序号
type ResourceConfig struct {
	MemoryLimit string
	CpuCfsQuota int
	CpuShare    string
	CpuSet      string
}
