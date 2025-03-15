package network

import (
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"text/tabwriter"
)

func CreateNetwork(driver, subnet, name string) error {
	// 将网段的字符创转换成 net.IPNet 的对象
	_, cidr, _ := net.ParseCIDR(subnet)
	// 通过 IPAM 分配网关IP，获取到网段中第一个 IP 作为网关的 IP
	// 以 192.168.1.0/24 为例，此处获得的 IP 应该为 192.168.1.1，将该地址作为网关
	ip, err := IPAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	// cidr 中除了 IP 还有掩码信息 24
	// 此处 cidr 应该为 192.168.1.1/24
	cidr.IP = ip
	// 调用指定的网络驱动创建网络，这里的 drivers 字典是各个网络驱动的实例字典
	// 通过调用网络驱动的 Create 方法创建网络，后面会议 Bridge 驱动为例介绍它的实现
	net, err := Drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}
	// 保存网络信息到宿主机指定文件中，以便查询，以及在网络上连接网络端点
	return net.dump(defaultNetworkPath)
}

// ListNetwork 打印出当前全部 Network 信息
func ListNetwork() {
	networks, err := LoadNetwork()
	if err != nil {
		log.Errorf("load network from file failed, detail: %v", err)
		return
	}
	// 通过 tabwriter 库 把信息打印到屏幕上
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tIPRange\tDriver\n")
	for _, net := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			net.Name,
			net.IPRange.String(),
			net.Driver,
		)
	}
	if err = w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

func DeleteNetwork(networkName string) error {
	networks, err := LoadNetwork()
	if err != nil {
		// 如果只是想添加额外信息（不需要堆栈），用 errors.WithMessage。
		// 如果想要记录错误的调用栈（方便调试），用 errors.Wrapf
		return errors.WithMessage(err, "load network from file failed")
	}
	// 网络不存在直接返回一个 error
	net, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no Such Network: %s", networkName)
	}
	// 调用 IPAM 的实例 ipAllocator 释放网络网关的 IP
	log.Debugf("DeleteNetwork net info: IPRange: %s, IP: %s", net.IPRange.String(), net.IPRange.IP.String())
	if err = IPAllocator.Release(net.IPRange, &net.IPRange.IP); err != nil {
		return errors.Wrap(err, "remove Network gateway ip failed")
	}
	// 调用网络驱动删除网络创建的设备与配置，后面会以 Bridge 驱动删除网络为例，介绍如何实现网络驱动删除网络
	if err = Drivers[net.Driver].Delete(net); err != nil {
		return errors.Wrap(err, "remove Network Driver Error failed")
	}
	// 最后从网络的配置目录中删除该网络对应的配置文件
	return net.remove(defaultNetworkPath)
}
