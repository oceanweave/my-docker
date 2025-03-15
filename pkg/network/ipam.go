package network

import (
	"encoding/json"
	"fmt"
	"github.com/oceanweave/my-docker/pkg/constant"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"path"
	"strings"
)

/*
bitmap 在大规模连续且少状态的数据处理中有很高的效率，比如要用到的 IP 地址分配。
一个网段中的某个 IP 地址有两种状态：
- 1 表示已经被分配了，
- 0 表示还未被分配；
那么一个 IP 地址的状态就可以用一位来表示， 并且通过这位相对基础位的偏移也能够迅速定位到数据所在的位。
通过位图的方式实现 IP 地址的管理也比较简单：
- 分配 IP：在获取 IP 地址时，遍历每一项，找到值为 0 的项的偏移，然后通过偏移和网段的配置计算出分配的 IP 地址，并将该位置元素置为 1，表明 IP 地址已经被分配。
- 释放 IP：根据 IP 和网段配置计算出偏移，然后将该位置元素置为 0，表示该 IP 地址可用。
*/

const ipamDefaultAllocatorPath = "/var/lib/mydocker/network/ipam/subnet.json"

type IPAM struct {
	SubnetAllocatorPath string             // 分配文件存放地址
	Subnets             *map[string]string // 网段和位图算法的数组 map，key 是网段，value 是分配的位图数组
}

// 初始化一个 IPAM 的对象，默认使用 /var/lib/mydocker/network/ipam/subnet.json
// 将字母改为大写，便于别的包引用
var IPAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

// 读取文件数据到内存
// load 加载网段地址分配信息
func (ipam *IPAM) load() error {
	// 检查存储文件状态，若不存在，则说明之前没有分配，就不需要加载
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// 文件不存在的错误，返回的 error 为 nil
		return nil
	}
	log.Debugf("Try load Network-IPAM-ConfigFile: %s", ipam.SubnetAllocatorPath)
	// 读取文件，加载配置信息
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	if err != nil {
		return err
	}
	defer subnetConfigFile.Close()
	// 读取缓冲区
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return errors.Wrap(err, "read subnet config file error")
	}
	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	return errors.Wrap(err, "err load allocation info")
	/*
		补充知识
				Read()
				- 需要先 os.Open()，然后 Read(buf) 逐步读取
				- 需要 defer file.Close()
				- 适合大文件，手动控制读取大小
				- 内存占用低，可按需读取
				- 流式处理（日志、解析数据）
				ReadFile()
				- 直接 os.ReadFile("file.txt") 一次性读取
				- 不需要 defer 手动关闭
				- 适合小文件，一次性加载整个文件
				- 内存占用高，大文件可能导致 OOM
				- 快速获取整个文件内容
			Read() 示例
			func main() {
				// 打开文件
				file, err := os.Open("example.txt")
				if err != nil {
					fmt.Println("打开文件失败:", err)
					return
				}
				defer file.Close()

				// 创建缓冲区
				buf := make([]byte, 1024) // 1KB 缓冲区

				// 逐步读取文件
				for {
					n, err := file.Read(buf) // 读取 1024 字节
					if err != nil {
						break
					}
					fmt.Print(string(buf[:n])) // 打印读取的内容
				}
			}
	*/
}

// dump 存储网段地址分配信息
func (ipam *IPAM) dump() error {
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err = os.MkdirAll(ipamConfigFileDir, constant.Perm0644); err != nil {
			return err
		}
	}
	// 打开存储文件 O_TRUNC 表示如果存在则消空； O_CREATE 表示如果不存在则创建
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, constant.Perm0644)
	if err != nil {
		return err
	}
	defer subnetConfigFile.Close()
	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}
	_, err = subnetConfigFile.Write(ipamConfigJson)
	log.Debugf("Dump Network-IPAM-Info to File: %s", ipam.SubnetAllocatorPath)
	return err
}

/*
	1）从文件中加载 IPAM 数据
	2）根据子网信息在 map 中找到存储 IP 分配信息的字符串
	3）遍历字符串找到其中为 0 的元素，并根据偏移按照算法计算得到本次分配的 IP
	4）把对应位置置 1 并写回文件
*/
// Allocate 在网段中分配一个可用的 IP 地址
func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	log.Debugf("Allocate IP ... ...")
	// 存放网段中地址分配信息的数组
	ipam.Subnets = &map[string]string{}

	// 从文件中加载已经分配的网段信息，文件不存在也不会报错；有就加载
	err = ipam.load()
	if err != nil {
		return nil, errors.Wrap(err, "load subnet allocation infor error")
	}
	/*
		1. net.ParseCIDR(string) 解析一个 CIDR 格式的 IP 地址，例如 "192.168.1.0/24"。
		返回 三个值：
		- IP（被 _ 忽略）
		- *net.IPNet（子网信息，赋值给 subnet）
		- error（被 _ 忽略
		2. subnet.Mask.Size()
		subnet.Mask 表示 子网掩码，例如 255.255.255.0。
		.Size() 返回 两个整数：
		- one（子网掩码中 1 的个数，例如 /24 就是 24）
		- size（子网掩码总长度，IPv4 是 32，IPv6 是 128）
	*/
	_, subnet, _ = net.ParseCIDR(subnet.String())
	one, size := subnet.Mask.Size()

	// 若之前没分配这个网段，则初始化该网段的分配配置
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		// 生成该网段的初始 IP 位图
		// 1<<uint8(size-one) 表示 2 的 size-one 次幂
		// 生成 N 个 "0" 组成的字符串，用于标记该子网的 IP 分配状态; 初始时，所有 IP 都是 "0"，表示 未分配
		// /24 → strings.Repeat("0", 256) → "000000...000" (共 256 个 0) —— /24 对应 one=24  size=32  2的（32-24）次幂=256

		// 在一个子网范围内：
		// 1. 网段地址（Network Address）：子网的第一个地址，用于标识整个网络，不可分配给主机。
		// 2. 广播地址（Broadcast Address）：子网的最后一个地址，用于向整个子网广播数据包，也不可分配给主机。
		// 3. 所以可分配地址数量为 1<<uint8(size-one) - 2 表示该网段可分配的数量
		// 4. 网关网段内第一个可用地址，一般作为网关
		/*
			以 192.168.1.0/24 网段为例
			- 192.168.1.0 → 网段地址（Network Address，不可分配）,用于标识整个 192.168.1.0/24 网络，不能分配给设备,它常见于路由表、子网定义、IP 规划、DHCP 配置和路由协议中。
			- 192.168.1.1 → 通常分配给网桥，作为网关
			- 192.168.1.2 ~ 192.168.1.254 → 可分配给主机
			- 192.168.1.255 → 广播地址（Broadcast Address，不可分配）
		*/
		ipCount := 1 << uint8(size-one)
		ipalloc := strings.Repeat("0", ipCount-2)
		(*ipam.Subnets)[subnet.String()] = fmt.Sprintf("1%s1", ipalloc)
	}

	// 遍历网段的位图数组
	for freePos := range (*ipam.Subnets)[subnet.String()] {
		if (*ipam.Subnets)[subnet.String()][freePos] == '0' {
			ipAlloc := []byte((*ipam.Subnets)[subnet.String()])
			ipAlloc[freePos] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipAlloc)
			// 获取网段首 IP ，如 172.16.0.0/24 取得 172.16.0.0
			ip = subnet.IP
			// ip 根据点分割为 4 部分，freePos 是个十进制数，将其转换为二进制数，再分割为 4 部分，这几部分与网段首 ip 相加，就是获取到的 ip
			// 这里表示 freePos 分别位移 24、16、8，然后 uint8 相当于模256，就是取低 8 位，得到的结果就是每部分的 偏移量
			/*
				还需要通过网段的IP与上面的偏移相加计算出分配的IP地址，由于IP地址是uint的一个数组，
				需要通过数组中的每一项加所需要的值，比如网段是172.16.0.0/12，数组序号是65555,
				那么在[172,16,0,0] 上依次加[uint8(65555 >> 24)、uint8(65555 >> 16)、
				uint8(65555 >> 8)、uint8(65555 >> 0)]， 即[0, 1, 0, 19]， 那么获得的IP就
				是172.17.0.19.
			*/
			// 对每个部分进行模 256，保证 ip 由 . 分割的4个部分都在 0-255 内
			for t := uint(4); t > 0; t-- {
				// uint8（无符号 8 位整数）的范围是 0~255，而 256 超出了 uint8 的范围。 当一个大于 255 的值转换为 uint8 时，会 取模 256
				[]byte(ip)[4-t] += uint8(freePos >> ((t - 1) * 8))
			}
			break
		}
	}
	err = ipam.dump()
	if err != nil {
		log.Error("Allocate: dump ipam error", err)
	}
	log.Debugf("Allocate IP Finish —— subnet: %s, get free ip: %s", subnet.String(), ip)
	return
}

// Release 从 subnet 解析出真正的网段， ipaddr 为待释放的 ip
func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	log.Debugf("Release IP ... ...")
	ipam.Subnets = &map[string]string{}
	// 从 subnet 解析出真正的网段， subnet 为 192.168.0.1/24 此处解析出的 subnet 为 192.168.0.1/24
	_, subnet, _ = net.ParseCIDR(subnet.String())

	err := ipam.load()
	if err != nil {
		return errors.Wrap(err, "load subnet allocation info error")
	}
	// 和分配一样的算法，反过来根据 ip 找到位图数组中的对应索引位置
	needReleasePos := 0
	releaseIP := ipaddr.To4()
	for t := uint(4); t > 0; t-- {
		// 每个部分的偏移量，要移动回去
		needReleasePos += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}
	// 然后将对应的位置 置为0
	ipAlloc := []byte((*ipam.Subnets)[subnet.String()])
	ipAlloc[needReleasePos] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipAlloc)

	// 最后调用 dump 将分配结果保存到文件中
	err = ipam.dump()
	if err != nil {
		log.Error("Allocate: dump ipam error", err)
	}
	log.Debugf("Release IP Finish —— subnet: %s, release ip: %s", subnet.String(), ipaddr.String())
	return nil
}
