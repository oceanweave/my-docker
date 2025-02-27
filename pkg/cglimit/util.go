package cglimit

import "github.com/oceanweave/my-docker/pkg/cglimit/types"

func IsSetResource(res *types.ResourceConfig) bool {
	if res.MemoryLimit != "" || res.CpuSet != "" || res.CpuShare != "" || res.CpuCfsQuota > 0 {
		return true
	}
	return false
}
