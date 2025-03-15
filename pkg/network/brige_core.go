package network

import (
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"strings"
	"time"
)

/*
# 1. 创建网桥
sudo brctl addbr br0
# 2. 为bridge分配IP地址，激活上线
sudo ip addr add 172.18.0.1/24 dev br0
# 3. 启动网桥设备
sudo ip link set br0 up
# 4. 配置 nat 规则让容器可以访问外网， SNAT 将源ip  172.18.0.0/24 转为 宿主机ip
sudo iptables -t nat -A POSTROUTING -s 172.18.0.0/24 -o eth0 -j MASQUERADE

# -o br0：表示数据包 通过 br0 这个网桥接口 发送。 !（取反）：表示 不通过 br0 发送的数据包 才会匹配此规则。
# 对 源地址为 172.18.0.0/24 的数据包进行 NAT； 但 不适用于走 br0 接口的数据包，只有 从其他接口（如 eth0、wlan0）出去 的流量才会被修改
# 让 172.18.0.0/24 的容器 可以通过 eth0 访问外网；但 不会影响 br0 连接的本地通信，避免 NAT 影响局域网通信
# 这条命令可以简单理解为： 符合该网段，去往 br0 网桥的流量不需要进行 SNAT（也就是容器网桥内部不需要进行 SNAT)
sudo iptables -t nat -A POSTROUTING -s 172.18.0.0/24 ! -o br0 -j MASQUERADE
# 如果想让 172.18.0.0/24 网段的设备 只能通过 eth0 访问外网，可以更明确地指定
sudo iptables -t nat -A POSTROUTING -s 172.18.0.0/24 -o eth0 -j MASQUERADE
*/

func (b *BridgeNetworkDriver) initBridge(n *Network) error {
	bridgeName := n.Name
	// 1. 创建 Bridge 虚拟设备
	if err := createBridgeInterface(bridgeName); err != nil {
		return errors.Wrapf(err, "Failed to create bridge %s", bridgeName)
	}
	// 2. 设置 Bridge 设备地址和路由
	gatewayIP := *n.IPRange
	log.Debugf("initBridge gatewayIP: %s", gatewayIP.String())
	// gatewayIP.String() 返回形式如 "192.168.1.1/24"
	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return errors.Wrapf(err, "Error set bridge ip: %s on bridge: %s", gatewayIP.String(), bridgeName)
	}
	// 3. 启动 Bridge 设备
	if err := setInterfaceUP(bridgeName); err != nil {
		return errors.Wrapf(err, "Failed to set %s up", bridgeName)
	}
	// 4. 设置 iptables SNAT 规则
	if err := setupIPTables(bridgeName, n.IPRange); err != nil {
		return errors.Wrapf(err, "Failed to set up iptables for %s", bridgeName)
	}
	// 5. 设置路由规则 报错但配置上（是默认配置的吗？）
	// 问题解答：
	// - 若容器访问外网，需要在宿主机上配置（可转发） sysctl net.ipv4.conf.all.forwarding=1
	// - 配置上面，创建网桥时自动会为网桥增加一条路由，如 10.0.0.0/24 dev testbridge proto kernel scope link src 10.0.0.1
	// - 因此，此处再次配置，将会报错，所以无需配置（但是网桥删除的时候，需要手动删除此路由）
	//if err := setupRoutes(bridgeName, n.IPRange); err != nil {
	//	return errors.Wrapf(err, "Failed to set up Routes for %s", bridgeName)
	//}
	return nil
}

// 1. 创建 Bridge 虚拟设备, 相当于 ip link add xxxx
func createBridgeInterface(bridgeName string) error {
	// 先检查是否己经存在了这个同名的Bridge设备
	_, err := net.InterfaceByName(bridgeName)
	// 如果已经存在或者报错则返回创建错
	// errNoSuchInterface这个错误未导出也没提供判断方法，只能判断字符串了。。
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return nil
	}
	devAttr := netlink.NewLinkAttrs()
	devAttr.Name = bridgeName
	// 使用刚才创建的Link的属性创netlink Bridge对象
	br := &netlink.Bridge{LinkAttrs: devAttr}
	// 调用 net link Linkadd 方法，创 Bridge 虚拟网络设备
	// netlink.LinkAdd 方法是用来创建虚拟网络设备的，相当于 ip link add xxxx
	if err = netlink.LinkAdd(br); err != nil {
		return errors.Wrapf(err, "create bridge %s error", bridgeName)
	}
	return nil
}

// 2. 设置 Bridge 设备地址和路由, 相当于ip addr add xxx命令 (ip addr add 172.18.0.1/24 dev br0)
func setInterfaceIP(name string, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error
	for i := 0; i < retries; i++ {
		// 通过LinkByName方法找到需要设置的网络接口
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new bridge netlink link [%s]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return errors.Wrap(err, "abandoning retrieving the new bridge link from netlink, Run [ip link] to troubleshoot")
	}
	// 由于 netlink.ParseIPNet 是对 net.ParseCIDR一个封装，因此可以将 net.PareCIDR 中返回的IP进行整合
	// 返回值中的 ipNet 既包含了网段的信息，192 168.0.0/24 ，也包含了原始的IP 192.168.0.1
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	// 通过  netlink.AddrAdd给网络接口配置地址，相当于ip addr add xxx命令
	// 同时如果配置了地址所在网段的信息，例如 192.168.0.0/24
	// 还会配置路由表 192.168.0.0/24 转发到这 testbridge 的网络接口上
	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(iface, addr)
}

// 3. 启动 Bridge 设备, 相当于 ip link set xxx up这个命令
func setInterfaceUP(interfaceName string) error {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return errors.Wrapf(err, "error retrieving a link named [%s]:", link.Attrs().Name)
	}
	// 等价于 ip link set xxx up 命令
	if err = netlink.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "enabling interface for %s", interfaceName)
	}
	return nil
}

// 4. 设置 iptables SNAT 规则
// 4.1  增加 iptables 规则，用于该网段的容器发往外部的数据包（将容器 IP SNAT 为 宿主机IP）
// $ iptables -t nat -A POSTROUTING -s 172.18.0.0/24 -o eth0 -j MASQUERADE
// # 语法：iptables -t nat -A POSTROUTING -s {subnet} -o {deviceName} -j MASQUERADE
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	return configIPTables(bridgeName, subnet, false)
}

// 4.2  删除 iptables 规则，当删除该网桥网络时，删除对应的 iptables 规则
// $ iptables -t nat -D POSTROUTING -s 172.18.0.0/24 -o eth0 -j MASQUERADE
func deleteIPTables(bridgeName string, subnet *net.IPNet) error {
	return configIPTables(bridgeName, subnet, true)
}

func configIPTables(bridgeName string, subnet *net.IPNet, isDelete bool) error {
	action := "-A"
	if isDelete {
		action = "-D"
	}
	// 拼接命令
	iptablesCmd := fmt.Sprintf("-t nat %s POSTROUTING -s %s ! -o %s -j MASQUERADE", action, subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	log.Infof("Add(-A) or Delete(-D) SNAT cmd：%v", cmd.String())
	// 执行该命令
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}
	return err
}

// 5. 在宿主机上设置路由，发往此网段的数据包都通过该网桥发送
// 示例 10.0.0.0/24 dev testbridge proto kernel scope link src 10.0.0.1
func setupRoutes(bridgeName string, subnet *net.IPNet) error {
	// 目标网络
	bridgeIP, dstIPRanage, err := net.ParseCIDR(subnet.String())
	if err != nil {
		fmt.Println("Error parsing CIDR:", err)
		return err
	}

	// 查找网桥
	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		fmt.Println("Error finding link:", err)
		return err
	}

	// 设置路由
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index, // 设备索引
		Dst:       dstIPRanage,        // 目标网络
		Scope:     netlink.SCOPE_LINK, // 链路级别的作用域
		Src:       bridgeIP,           // 指定 src 地址
	}

	// 添加路由
	if err := netlink.RouteAdd(route); err != nil {
		return errors.Wrapf(err, "Error adding route:")
	}
	return nil
}

// 6. 删除路由
// - ip route del 10.0.0.0/24 dev testbridge，
// - 如果还想确保 src IP 也匹配 ip route del 10.0.0.0/24 dev testbridge src 10.0.0.1
func deleteIPRoute(name string, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error
	for i := 0; i < retries; i++ {
		// 通过LinkByName方法找到需要设置的网络接口
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return errors.Wrap(err, "abandoning retrieving the new bridge link from netlink, Run [ ip link ] to troubleshoot")
	}
	// 查询对应设备的路由并全部删除
	list, err := netlink.RouteList(iface, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, route := range list {
		if route.Dst.String() == rawIP { // 根据子网进行匹配
			err = netlink.RouteDel(&route)
			if err != nil {
				log.Errorf("route [%v] del failed,detail:%v", route, err)
				continue
			}
		}
	}
	return nil
}
