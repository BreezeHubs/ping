package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"time"
)

var (
	timeout      int64                 //超时时间
	count        int                   //请求次数
	size         int                   //缓冲区大小
	sendCount    int                   //已发起请求次数
	successCount int                   //成功请求次数
	failCount    int                   //失败请求次数
	minTs        int64 = math.MaxInt32 //最小耗时，设置可计数的默认最大取值范围，以int32划分
	maxTs        int64 = 0             //最大耗时
	totalTs      int64                 //总耗时
)

// ICMP icmp数据结构
type ICMP struct {
	Type     uint8  //icmp报文type
	Code     uint8  //code
	CheckSum uint16 //校验和
	ID       uint16 //ID
	SeqNum   uint16 //序号
}

func main() {
	getArgs()              //初始化命令行参数
	host := getArgOfHost() //取最后一个参数
	ping(host)             //ping
}

func ping(host string) {
	conn, err := net.DialTimeout(
		"ip4:icmp", //协议
		host,
		time.Duration(timeout)*time.Millisecond, //毫秒
	)
	if err != nil {
		fmt.Printf("Ping 请求找不到主机 %s。请检查该名称，然后重试。", host)
		os.Exit(0)
	}
	defer conn.Close()

	fmt.Printf("正在 Ping %s [%s] 具有 %d 字节的数据：\n", host, conn.RemoteAddr(), size)

	for i := 0; i < count; i++ {
		sendCount++ //统计请求数

		//定义icmp数据
		icmp := &ICMP{
			Type:     8,         //icmp报文type为8位
			Code:     0,         //code 8位
			CheckSum: 0,         //校验和 16位
			ID:       uint16(i), //ID 16位
			SeqNum:   uint16(i), //序号 16位
		}

		//创建缓冲区，以大端方式写入icmp头部
		//binary.BigEndian（大端模式）：内存的低地址存放着数据高位
		//binary.LittleEndian(小端模式)：内存的低地址存放着数据低位
		var buffer bytes.Buffer
		binary.Write(&buffer, binary.BigEndian, icmp)

		//声明icmp内容部分
		data := make([]byte, size)
		buffer.Write(data)
		data = buffer.Bytes()

		//检验和
		checkSum, err := checkSum(data)
		if err != nil {
			failCount++
			continue
		}

		data[2] = byte(checkSum >> 8) //code，高位
		data[3] = byte(checkSum)      //checksum，地位

		//设置传输超时时间
		conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))

		tStart := time.Now() //用于统计时间

		//传输
		if _, err = conn.Write(data); err != nil {
			failCount++
			fmt.Println("请求失败。")
			continue
		}

		buf := make([]byte, 1<<16) //65535
		n, err := conn.Read(buf)   //接收返回数据

		//计算时间
		tSpend := time.Since(tStart).Milliseconds()
		totalTs += tSpend   //累计总花费时间
		if minTs > tSpend { //最小花费时间
			minTs = tSpend
		}
		if maxTs < tSpend { //最大花费时间
			maxTs = tSpend
		}

		if err != nil {
			failCount++
			fmt.Println("请求超时。")
			continue
		}
		successCount++ //统计成功请求数
		fmt.Printf("来自 %d.%d.%d.%d 的回复: 字节=%d 时间=%dms TTL=%d\n", buf[12], buf[13], buf[14], buf[15], n-28, tSpend, buf[8])
	}

	//输出总结
	fmt.Printf("\n%s 的 Ping 统计信息:\n    数据包: 已发送 = %d，已接收 = %d，丢失 = %d (%.2f%% 丢失)，\n往返行程的估计时间(以毫秒为单位):\n    最短 = %dms，最长 = %dms，平均 = %dms\n",
		conn.RemoteAddr(), sendCount, successCount, failCount, float64(failCount)/float64(sendCount), minTs, maxTs, totalTs/int64(sendCount))
}

// 检验和算法
// 1、报文内容，相邻两个字节拼接到一起组成一个16bit的数，将这些数累加
// 2、若长度为奇数，则将剩余的1个字节直接累加
// 3、得到总和后，将该值的高16位与低16位不断求和，直到高16位为0
// 4、最后的和取反，就为校验和
func checkSum(data []byte) (uint16, error) {
	len := len(data)
	idx := 0
	var sum uint32
	for len > 1 {
		sum += uint32(data[idx])<<8 + uint32(data[idx+1]) //相邻两位拼接，第一个数向左移动8位，才能拼接第二个数
		len -= 2
		idx += 2
	}
	if len == 1 {
		sum += uint32(data[idx])
	}

	//sum最大值：0xffffffff 16进制
	//高16位：0xffff
	//低16位：0xffff
	hi16 := sum >> 16
	for hi16 != 0 {
		sum = hi16 + uint32(uint16(sum))
		hi16 = sum >> 16
	}

	return uint16(^sum), nil
}

// 初始化命令行参数
func getArgs() {
	flag.Int64Var(&timeout, "w", 1000, "等待每次回复的超时时间(毫秒)")
	flag.IntVar(&count, "n", 4, "要发送的回显请求数")
	flag.IntVar(&size, "l", 32, "发送缓冲区大小")
	flag.Parse()
}

// 取最后一个参数
func getArgOfHost() string {
	if len(os.Args) < 2 {
		fmt.Println(`用法: ping [-n count] [-l size] [-w timeout] target_name

选项:
   -n count       要发送的回显请求数。
   -l size        发送缓冲区大小。
   -w timeout     等待每次回复的超时时间(毫秒)。`)
		os.Exit(0)
	}
	return os.Args[len(os.Args)-1]
}
