package subsystems

import "github.com/oceanweave/my-docker/pkg/cglimit/types"

// SubsystemsIns 通过不同的subsystem初始化实例创建资源限制处理链数组
var SubsystemsIns = []types.Subsystem{
	//&CpusetSubSystem{},
	&MemorySubSystem{},
	//&CpuSubSystem{},
}
