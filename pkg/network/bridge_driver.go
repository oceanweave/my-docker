package network

import (
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"net"
)

/*
这里实现简单的桥接网络作为容器的网络驱动，因此：
- Create：创建 Bridge 设备
- Delete：删除 Bridge 设备
- Connect：将 veth 关联到网桥
- Disconnect：将 veth 从网桥解绑
*/

type BridgeNetworkDriver struct {
}

func (b *BridgeNetworkDriver) Name() string {
	return "bridge"
}

// Create 根据子网信息创建 Bridge 设备并初始化
// 传入的参数，例如为 192.168.0.1/24  string  类型，此处就是将其转为 *net.IPNet 类型，存入到 Network 结构体中
func (b *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	// 此作用就是 192.168.0.1/24 可以解析 ip 为  192.168.0.1 ；解析 ipRange 为 192.168.0.0/24
	ip, ipRange, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, err
	}
	// 因此此处很关键，就是将 ipRange 中 ip 部分的 192.168.0.0 改为 192.168.0.1
	// 此处的 ip 后续要作为网桥的 ip 进行配置
	ipRange.IP = ip
	n := &Network{
		Name:    name,
		IPRange: ipRange,
		Driver:  b.Name(),
	}
	// 重点部分，创建 Bridge，配置 ip ，启动，配置 iptables 等
	err = b.initBridge(n)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create bridge network")
	}
	return n, err
}

// Delete 删除对应名称的 Bridge 设备即可
func (b *BridgeNetworkDriver) Delete(driverName string) error {
	// 根据名字找到对应的Bridge设备
	br, err := netlink.LinkByName(driverName)
	if err != nil {
		return err
	}
	// 删除网络对应的 Linux Bridge 设备
	err = netlink.LinkDel(br)
	if err != nil {
		log.Errorf("Error Delete Bridge [%s]", driverName)
		return err
	}
	return nil
}

/*
1. 创建 Veth 设备 —— veth1（宿主机端）cif-veth1（容器端）
ip link add veth1 type veth peer name cif-veth1
2. 绑定 veth1 到 Bridge
ip link set veth1 master br0
3. 启动 veth1（宿主机端）—— Go 代码的 netlink.LinkSetUp() 只执行了这一条命令。
ip link set veth1 up

4. 启动 cif-veth1（容器端）—— 此处未启动，Go 代码中没有这个操作，所以 容器端 veth 设备默认是 DOWN 状态
ip link set cif-veth1 up
*/

func (b *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	// 1. 通过接口名获取到 Linux Bridge 接口的对象和接口属性
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	// 2. 创建 Veth 接口的配置
	vethAttr := netlink.NewLinkAttrs()
	// 由于 Linux 接口名的限制,取 endpointID 的前5位
	// 设置 veth peer 一端名称（宿主机端），也就是加入到 bridge 的 veth
	vethAttr.Name = endpoint.ID[:5]
	// 通过设置 Veth 接口 master 属性，设置这个Veth的一端挂载到网络对应的 Linux Bridge
	vethAttr.MasterIndex = br.Attrs().Index
	// 语法糖：
	// endpoint 是指针，但 Go 自动解引用，所以 endpoint.Device 直接访问不需要 (*endpoint).Device
	// 创建 Veth 对象，通过 PeerName 配置 Veth 另外 端的接口名
	// 配置 Veth 另外一端的名字 cif {endpoint ID 的前 5 位｝
	endpoint.Device = netlink.Veth{
		LinkAttrs: vethAttr,
		PeerName:  "cif-" + endpoint.ID[:5], // 设置 veth peer 另一端名称（容器端）
	}
	// 3. 调用netlink的LinkAdd方法创建出这个Veth接口
	// 因为上面指定了link的MasterIndex是网络对应的Linux Bridge
	// 所以Veth的一端就已经挂载到了网络对应的LinuxBridge.上
	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	// 4. 调用netlink的LinkSetUp方法，设置Veth启动，启动宿主机端的 veth
	// 相当于ip link set xxx up命令
	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("error SetUp Endpoint Device: %v", err)
	}
	return nil
}

/*
1. 获取 Veth 设备
ip link show veth1
2. 解除 Veth 设备与 Bridge 的绑定
ip link set veth1 nomaster
3. 删除 Veth 设备 —— Veth 设备是成对的，这会导致它的对端 cif-veth1 也被删除
ip link delete veth1
*/

func (b *BridgeNetworkDriver) Disconnect(endpoint *Endpoint) error {
	// 1. 获取 Veth 设备
	vethName := endpoint.ID[:5]
	veth, err := netlink.LinkByName(vethName)
	if err != nil {
		return err
	}
	// 2. 解除 Veth 设备与 Bridge 的绑定
	err = netlink.LinkSetNoMaster(veth)
	if err != nil {
		return err
	}
	//// 3. 删除 Veth 设备 —— Veth 设备是成对的，这会导致它的对端 cif-veth1 也被删除
	//err = netlink.LinkDel(veth)
	//if err != nil {
	//	return err
	//}
	return nil
}
