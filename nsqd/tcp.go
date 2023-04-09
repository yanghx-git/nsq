package nsqd

import (
	"io"
	"net"
	"sync"

	"github.com/nsqio/nsq/internal/protocol"
)

const (
	typeConsumer = iota
	typeProducer
)

type Client interface {
	Type() int
	Stats(string) ClientStats
}

type tcpServer struct {
	nsqd *NSQD
	//存储客户端的连接 map  sync.Map  并发map
	conns sync.Map
}

// Handle 处理 TCP 连接的方法
func (p *tcpServer) Handle(conn net.Conn) {
	p.nsqd.logf(LOG_INFO, "TCP: new client(%s)", conn.RemoteAddr())

	// 4字节的魔数.表示协议的版本号
	// The client should initialize itself by sending a 4 byte sequence indicating
	// the version of the protocol that it intends to communicate, this will allow us
	// to gracefully upgrade the protocol away from text/line oriented to whatever...
	buf := make([]byte, 4)
	// 会阻塞，直到读取够4字节  (  V2)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		p.nsqd.logf(LOG_ERROR, "failed to read protocol version - %s", err)
		conn.Close()
		return
	}
	protocolMagic := string(buf)

	p.nsqd.logf(LOG_INFO, "CLIENT(%s): desired protocol magic '%s'",
		conn.RemoteAddr(), protocolMagic)

	var prot protocol.Protocol
	switch protocolMagic {
	case "  V2":
		// 初始化
		prot = &protocolV2{nsqd: p.nsqd}
	default:
		// 协议错误，给个提示信息，结束连接
		protocol.SendFramedResponse(conn, frameTypeError, []byte("E_BAD_PROTOCOL"))
		conn.Close()
		p.nsqd.logf(LOG_ERROR, "client(%s) bad protocol magic '%s'",
			conn.RemoteAddr(), protocolMagic)
		return
	}
	// 对新建立的连接创建 client
	client := prot.NewClient(conn)
	// 存储 连接。 放到 map中
	p.conns.Store(conn.RemoteAddr(), client)

	// io处理 (业务)
	err = prot.IOLoop(client)
	if err != nil {
		p.nsqd.logf(LOG_ERROR, "client(%s) - %s", conn.RemoteAddr(), err)
	}

	p.conns.Delete(conn.RemoteAddr())
	// 下面一行等价 conn.Close()
	client.Close()
}

func (p *tcpServer) Close() {
	p.conns.Range(func(k, v interface{}) bool {
		v.(protocol.Client).Close()
		return true
	})
}
