package network

import (
	"github.com/vishvananda/netlink"
	"net"
)

// Network 表示容器网络的相关配置，如网络的容器地址断、网络操作所调用的网络驱动等信息
// 存储网络驱动相关信息（如网桥）
type Network struct {
	// 用户指定的网络名称，比如 testbr0，此信息用于持久化该网络信息到对应的文件 /var/lib/mydocker/network/network/testbr0
	Name string // 网络名
	// 此处相当于将网段信息和网关信息混合了，比如表示为 192.168.0.1/24
	// ip, subnet, _ = net.ParseCIDR(IPRange.String())
	// 利用此方法解析出 ip 为 192.168.0.1 （网关IP），解析出 subnet 为 192.168.0.0/24（网段信息）
	// 另外注意 192.168.0.1/24 表示的是单个 IP （24只是表示掩码），只有首 IP 表示的是网段 也就是  192.168.0.0/24（网段信息）
	IPRange *net.IPNet // 地址段
	// 表示采用哪种驱动创建网络，目前只支持 bridge
	Driver string // 网络驱动名
}

// Endpoint 表示网络端点的相关信息，如地址、Veth设备、端口映射、连接的容器和网络等信息
// 存储容器相关信息（如 veth pair）
type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	Network     *Network
	PortMapping []string
}

// Driver 表示不同类型网络驱动的相同行为特征（对网络的创建、连接、销毁），不过具体执行的操作是不同的（视驱动实例而定）
type Driver interface {
	Name() string
	Create(subnet string, name string) (*Network, error)
	Delete(network *Network) error
	Connect(network *Network, endpoint *Endpoint) error
	Disconnect(endpoint *Endpoint) error
}

// IPAMer 用于网络 IP 地址的分配和释放，包括容器的 IP 地址和网络网关的 IP 地址
type IPAMer interface {
	Allocate(subnet *net.IPNet) (ip net.IP, err error)
	Relese(subnet *net.IPNet, ipaddr *net.IP) error
}
