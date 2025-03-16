package container

import (
	"fmt"
	"github.com/oceanweave/my-docker/pkg/network"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//func ReleaseContainerIP(containerInfo *ContainerInfo) {
//	ip, ipNet, _ := net.ParseCIDR("192.168.0.1/24")
//	err := IPAllocator.Release(ipNet, &ip)
//	if err != nil {
//		t.Fatal(err)
//	}
//}

func Connect(networkName string, containerInfo *ContainerInfo) error {
	log.Debugf("Set Container Network")
	// 1. 加载当前记录的所有 network 信息
	networks, err := network.LoadNetwork()
	//fmt.Println(networks)
	if err != nil {
		return errors.WithMessage(err, "load network frome file failed")
	}
	// 2. 根据容器配置的网络名称 networkName，从 networks 字典中取到容器连接的网络的信息，
	// networks 字典中保存了当前已经创建的所有网络
	net, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no Such Network: %s", networkName)
	}
	// 3. 根据获取到的 network 信息，为容器容器分配空闲的 IP 地址
	ip, err := network.IPAllocator.Allocate(net.IPRange)
	if err != nil {
		return errors.Wrapf(err, "allocate ip")
	}

	log.Debugf("ContainerInfo —— ContainerId: %s, UseNetwork: %s, ContainerIP: %s", containerInfo.Id, containerInfo.NetworkName, ip)

	// 4. 创建代码内部结构（网络端点 Endpoint），记录此容器相关的网络信息
	// 创建网络端点，记录容器id，容器 ip，使用的 network，容器的端口映射
	ep := &network.Endpoint{
		ID:          fmt.Sprintf("%s-%s", containerInfo.Id, networkName),
		IPAddress:   ip,
		Network:     net,
		PortMapping: containerInfo.PortMapping,
	}
	// 记录 container IP 和相应的 掩码信息

	// 5. 调用网络驱动挂载和配置网络端点
	// 创建 veth pair，将一端加入到网桥上并启动（通过 net 中信息可找到已创建的网桥）
	// veth pair 的另一端 暂未进行操作，后续会移动到 容器 net namespace 中
	if err = network.Drivers[net.Driver].Connect(net, ep); err != nil {
		return err
	}
	// 6. 到容器的 namespace 配置容器网络设备 IP 地址
	// - 将上面 veth pair 的另一端移入到容器的 net namespace 中
	// - 同时进入到容器 net namespace，配置 veth 设备为容器 ip 并启动，同时启动回环网卡，并配置网关路由（网关就是 network 设备 即网桥）
	// - 最后返回到宿主机 net namespace
	if err = configEndpointIpAddressAndRoute(ep, containerInfo); err != nil {
		return err
	}
	// 7. 配置端口映射信息，例如  mydocker run -p 8080:80 -p 8090:90
	// - 通过 iptables 实现宿主机端口到容器端口的映射（nat）
	// - 8080:80 第一个参数为宿主机端口 如 8080， 第二个参数为容器端口 如80
	if err = configPortMappping(ep); err != nil {
		return err
	}

	return nil
}

// configPortMapping 配置端口映射
func configPortMappping(ep *network.Endpoint) error {
	var err error
	log.Debugf("Start Set Contaienr PortMapping[%s]", ep.PortMapping)
	// 1. 遍历容器端口映射列表
	for _, pm := range ep.PortMapping {
		// 2. 分割成宿主机的端口和容器的端口
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			log.Errorf("port mappping format error, %v", pm)
			continue
		}
		// 3. 配置 iptables 是先宿主机端口到容器端口的映射
		// 由于 iptables 没有 Go 语言版本的实现，所以采用 exec.Command 的方式直接调用命令配置
		// 在 iptables 的 PREROUTING  中添加 DNAT 规则
		// 将宿主机的端口请求转发到容器的地址和端口上
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		// 执行 iptables 命令，添加端口映射转发规则
		output, err := cmd.Output()
		if err != nil {
			log.Errorf("Error Set iptables, Output: %v, Error: %s", output, err)
			continue
		}
		log.Debugf("Finsh Set Container[%s] PortMapping Set HostPort[%s] to ContainerPort[%s], Cmd[%s]", ep.ID, portMapping[0], portMapping[1], cmd.String())
	}
	return err
}

// configEndpointIpAddressAndRoute 配置容器网络端点的地址和路由
/*
获取容器进程 PID=12345
获取容器进程的 net namespace 空间 NS_FILE="/proc/$PID/ns/net"
宿主机上容器端 veth 设备名称 VETH_NAME="veth1234"
# 1. (此时在宿主机网络命名空间内）将 veth 设备移入容器网络命名空间
# 在宿主机中，veth 设备的名字可能是 veth1234，
# 但当 veth 的一端进入容器网络命名空间后，它会被重命名为 eth0(此代码目前没有进行重命名，容器内veth仍为 cif- 开头）
ip link set "$VETH_NAME" netns "$PID"

# 2. 进入容器的网络命名空间
nsenter --net="$NS_FILE"

# 3. 在容器网络命名空间内配置网络
# 容器端 veth 被放入 容器net namespace 内后，会被自动重名为 eht0
ip addr add 192.168.1.2/24 dev eth0
ip link set eth0 up
ip link set lo up

# 4. 退出容器网络命名空间（返回到宿主机网络命名空间内）
exit
*/
func configEndpointIpAddressAndRoute(ep *network.Endpoint, containerInfo *ContainerInfo) error {
	// 1. 根据容器端 veth 名字找到对应 Veth 设备（cif- 开头的）
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}
	// 将容器的网络端点加入到容器的网络空间中
	// 并是这个函数下面的操作都在这个容器网络空间内进行
	// 执行完函数后，恢复为默认的宿主机网络空间，具体实现下面再做介绍

	// 2. 进入到容器 net namespace 中，defer 匿名函数用于返回宿主机 net namespace
	// defer 会立即执行该函数 enterContainerNetNS ，进入到容器 net namespace 内
	// enterContainerNetNS 返回的匿名函数，会延迟执行（也就是 defer 会延迟执行该匿名函数）
	// 该匿名函数的作用是，返回到 宿主机 net namespace 中
	defer enterContainerNetNS(&peerLink, containerInfo)()

	// 3. 在容器 net namespace 内配置 veth IP 及 路由等
	// 由于 defer 执行了 enterContainerNetNS 进入到 容器 net namespace 内
	// 所以下面都是在容器 net namespace 内进行配置

	// 获取到的容器 IP 地址及玩孤单，用于配置容器内部接口地址
	// 比如容器 IP 是 192.168.1.2，而网络的网段是 192.168.1.0/24
	// 那么这里产出的 IP 字符串就是 192.168.1.2/24，用于容器内 Veth 端点配置

	// *ep.Network.IPRange 记录的网关信息和网段信息（24掩码），如 192.168.1.1/24
	interfaceIP := *ep.Network.IPRange
	// ep.IPAddress 记录的是容器 IP，如 192.168.1.2
	// 执行到此处，相当于 interfaceIP 为 192.168.1.2/24 (既有 IP 又有掩码）
	interfaceIP.IP = ep.IPAddress

	// 3.1 设置容器内 Veth 端点的 IP
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}
	// 3.2 启动容器内的 Veth 端点
	if err = setInterfaceUP(ep.Device.PeerName); err != nil {
		return err
	}
	log.Debugf("Finish Set Container[%s] Veth[%s] IP[%s], And Set UP", containerInfo.Id, ep.Device.PeerName, interfaceIP.IP.String())
	// 3.3 启动本地回环网卡
	// Net Namespace 中默认本地地址 127 的回环网卡是关闭状态的
	// 启动该网卡来保证容器访问自己的请求
	if err := setInterfaceUP("lo"); err != nil {
		return err
	}
	log.Debugf("Finish Set Container[%s] local NIC UP", containerInfo.Id)

	// 3.4 配置网关和路由
	// 设置容器内的外部请求都通过容器内的 Veth 端点出去
	// 0.0.0.0/0 的网段，表示所有的 IP 地址段
	// 0.0.0.0/0 代表所有 IPv4 地址，即默认路由，用于匹配所有出站流量。
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	// 构建要添加的路由数据，包括网络设备、网关 IP 及目的网段
	// ip route add default via [Bridge网桥IP] dev [容器Veth设备]
	// 相当于 route add -net 0.0.0.0/0 gw [Bridge网桥IP] dev [容器Veth设备]
	// dev [容器Veth设备] 指定了出口设备; via [Bridge网桥IP] 指定了下一跳网关，即 Bridge 设备的 IP 地址（宿主机上的 br0）
	// 也就是 默认通过 [容器Veth设备] 将数据包发送到 网关[Bridge网桥]

	defaultRoute := &netlink.Route{
		// 此处获取的 index 是容器 veth 在宿主机 net namespace 内的 index
		LinkIndex: peerLink.Attrs().Index, // 关联的 Veth 设备（容器内部的网络设备）
		Gw:        ep.Network.IPRange.IP,  // 容器的网关 IP（通常是 Bridge 网桥的 IP）
		Dst:       cidr,                   // 目标网段（0.0.0.0/0，表示默认路由）
	}

	// 调用 netlink 的 RouteAdd， 添加路由到容器的网络空间
	// RouteAdd 函数相当于 route add 命令
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}
	log.Debugf("Finish Set Container[%s] Gateway[%s] Route[%s],Net Link Veth[%s] Index[%d]", containerInfo.Id, ep.Network.IPRange.IP, cidr, peerLink.Attrs().Name, peerLink.Attrs().Index)
	return nil
}

// enterContainerNetNS 将容器的网络端点加入到容器的网络空间中
// 并锁定当前程序所执行的线程，使当前线程进入到容器的网络空间
// 返回值是一个函数指针，执行这个返回函数才会退出容器的网络空间，回归到宿主机的网络空间
func enterContainerNetNS(enLink *netlink.Link, containerInfo *ContainerInfo) func() {
	// 1. 找到容器的 Net Namespace(就是容器进程 id 对应的宿主机 pro 的 ent 文件）
	// /proc/[pid]/ns/net 打开这个文件的文件描述符就可以来操作 Net Namespace
	// 而 ContainerInfo 中的 PID，即容器在宿主机上映射的进程 ID
	// 它对应的 /proc/[pid]/ns/net 就是容器内部的 Net Namespace
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", containerInfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("error get container net namespace, %v", err)
	}
	nsFD := f.Fd()

	// 2. 锁定当前程序所执行的线程，避免跳转到其他 OS 线程上，影响当前的网络配置
	// - 不是为了阻止其他线程运行，而是为了让某个 Goroutine 绑定到一个固定的 OS 线程上
	// - 适用于需要线程局部状态的场景，比如cgo、网络命名空间、OpenGL等
	// - 不能阻止其他线程上的 Goroutine 运行，也不会让 Goroutine 独占整个 CPU。
	// 锁定当前 Goroutine 到当前 OS 线程，避免 Go 调度器将其移动到其他 OS 线程
	// 如果不锁定操作系统线程的话, Go 语言的 goroutine 可能会被调度到别的线程上去
	// 就不能保证一直在所需要操作的网络空间中，所以要先锁定当前程序所在的线程
	// 该处的作用
	// - 将当前 goroutine 与系统 os 线程绑定，之后通过 setns 将该 os 线程放入到容器 net namespace 内
	// 	 之后，在容器 net namespace 内进程操作
	// - 若此处不绑定，在 setns 进入容器 net namespace 后，goroutine 被调动与另一个 os 线程绑定，
	//	 而另一个 os 线程对应的并不是容器 net namespace（可能对应宿主机 net namespace 或其他容器的 net namespace）
	//   就会导致容器的 net namespace 配置错乱
	runtime.LockOSThread()

	// 3. 修改网络端点 Veth 的另外一端，将其移动到容器的 Net Namespace 中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns, %v", err)
	}

	// 知识点：Go函数的接收者是结构体或指针结构体可以自动转换，但是若是接口指针，那么必须先解引用，才能调用对应的方法
	// 此处 enLink *netlink.Link 的 enLink 是接口指针，因此无法直接调用函数（因此 enLink.Attrs() 会报错），需要先解开引用
	// 若此处 enLink 是 结构体或结构体指针，那么是可以直接调用函数方法的，可以使用 enLink.Attrs()
	enLinkName := (*enLink).Attrs().Name
	containerNetNSFile := fmt.Sprintf("/proc/%s/ns/net", containerInfo.Pid)
	log.Debugf("enterContainerNetNS Func —— Move [%s] to Contaienr[%s] net namespace[%s]", enLinkName, containerInfo.Id, containerNetNSFile)

	// 4. 获取当前的宿主机网络 namespace，用于配置容器 net namespace 后返回宿主机 net namespace
	origins, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// 5. 将当前OS线程放入到容器 net namespace内
	// 调用 netns.Set 方法，将当前OS线程加入容器的 Net Namespace 中
	// 相当于进入了容器 net namespace 内，后续配置相关路由等
	// - netns.Set 仅影响当前 OS 线程，不会影响整个进程的其他线程
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("error set netns, %v", err)
	}

	// 6. 用于返回到宿主机 net namespace 中
	// 在容器的网络空间完成配置后，调用此函数就可以将程序恢复到原生的 Net Namespace
	return func() {
		netns.Set(origins)
		origins.Close()
		runtime.UnlockOSThread()
		f.Close()
		// 此处相当于个闭包函数，外部调用此函数时，仍可以利用该函数外的 containerInfo 信息
		// 闭包：一个函数“捕获”了外部作用域的变量，即使外部作用域已经结束，该函数仍然可以使用这些变量。
		// - “闭”：指的是函数封闭（捕获）了外部变量，使得它们即使超出原作用域也能被访问。
		// - “包”：意味着函数和它捕获的变量形成了一个整体，这些变量不会被垃圾回收，直到闭包本身被释放。
		// - 简单理解： 闭 —— 把外部变量关进来了，包 —— 形成一个整体
		// - 此处的外部变量，指的是 containerInfo.Id， 运行到此处其实相当于该函数已经结束，也就是 containerInfo.Id 生命周期已经结束了，
		//	 但是由于将 containerInfo.Id 关闭了起来，因此外部调用此匿名函数，仍可以使用 containerInfo.Id 变量记录的信息
		log.Debugf("enterContainerNetNS Func —— Contaienr[%s] —— Return to Host net namespace", containerInfo.Id)
	}
}

// 设置 veth 设备地址和路由, 相当于ip addr add xxx命令 (ip addr add 172.18.0.1/24 dev br0)
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

// 启动 veth 设备, 相当于 ip link set xxx up这个命令
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
