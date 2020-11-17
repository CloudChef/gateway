package main

import (
	"crypto/tls"
	"encoding/binary"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

const (
	/* 心跳消息 */
	TYPE_HEARTBEAT = 0x07

	/* 认证消息，检测clientKey是否正确 */
	C_TYPE_AUTH = 0x01

	/* 代理后端服务器建立连接消息 */
	TYPE_CONNECT = 0x03

	/* 代理后端服务器断开连接消息 */
	TYPE_DISCONNECT = 0x04

	/* 代理数据传输 */
	P_TYPE_TRANSFER = 0x05

	/* 用户与代理服务器以及代理客户端与真实服务器连接是否可写状态同步 */
	C_TYPE_WRITE_CONTROL = 0x06

	//协议各字段长度
	LEN_SIZE = 4

	TYPE_SIZE = 1

	SERIAL_NUMBER_SIZE = 8

	URI_LENGTH_SIZE = 1

	//心跳周期，服务器端空闲连接如果60秒没有数据上报就会关闭连接
	HEARTBEAT_INTERVAL = 30

	TOKEN = ""
)

var proxyConfig *ProxyConfig
var httpClient *HttpClient

type LPMessageHandler struct {
	connPool    *ConnHandlerPool
	connHandler *ConnHandler
	clientKey   string
	die         chan struct{}
}

type Message struct {
	Type         byte
	SerialNumber uint64
	Uri          string
	Data         []byte
}

type ProxyConnPooler struct {
	addr string
	conf *tls.Config
}

func main() {
	log.Println("smartcmp-proxy-agent - help you expose a local server behind a NAT or firewall to the internet")
	var conf *tls.Config
	conf = &tls.Config{
		InsecureSkipVerify: true,
	}
	listnerInfo := register()
	addDefaultService()
	start(listnerInfo.clientKey, listnerInfo.url, listnerInfo.sslPort, conf)
}

func start(key string, ip string, port int, conf *tls.Config) {
	connPool := &ConnHandlerPool{Size: 100, Pooler: &ProxyConnPooler{addr: ip + ":" + strconv.Itoa(port), conf: conf}}
	connPool.Init()
	connHandler := &ConnHandler{}
	for {
		//cmd connection
		log.Println(key, ip, port)
		conn := connect(key, ip, port, conf)
		connHandler.conn = conn
		messageHandler := LPMessageHandler{connPool: connPool}
		messageHandler.connHandler = connHandler
		messageHandler.clientKey = key
		messageHandler.startHeartbeat()
		log.Println("start listen cmd message:", messageHandler)
		connHandler.Listen(conn, &messageHandler)
	}
}

func connect(key string, ip string, port int, conf *tls.Config) net.Conn {
	err_count := 0
	for {
		var conn net.Conn
		var err error
		p := strconv.Itoa(port)
		if conf != nil {
			conn, err = tls.Dial("tcp", ip+":"+p, conf)
		} else {
			conn, err = net.Dial("tcp", ip+":"+p)
		}
		if err != nil {
			log.Println("In func connect,error dialing", err.Error(), err_count)
			err_count += 1
			time.Sleep(time.Second * 5)
			if err_count > 3 {
				syscall.Exit(0)
			}
		} else {
			return conn
		}
	}
}

func (messageHandler *LPMessageHandler) Encode(msg interface{}) []byte {
	if msg == nil {
		return []byte{}
	}

	message := msg.(Message)
	uriBytes := []byte(message.Uri)
	bodyLen := TYPE_SIZE + SERIAL_NUMBER_SIZE + URI_LENGTH_SIZE + len(uriBytes) + len(message.Data)
	data := make([]byte, LEN_SIZE, bodyLen+LEN_SIZE)
	binary.BigEndian.PutUint32(data, uint32(bodyLen))
	data = append(data, message.Type)
	snBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(snBytes, message.SerialNumber)
	data = append(data, snBytes...)
	data = append(data, byte(len(uriBytes)))
	data = append(data, uriBytes...)
	data = append(data, message.Data...)
	return data
}

func (messageHandler *LPMessageHandler) Decode(buf []byte) (interface{}, int) {
	lenBytes := buf[0:LEN_SIZE] // [0:4] 与LanProxyServer约定的通信协议
	bodyLen := binary.BigEndian.Uint32(lenBytes)
	if uint32(len(buf)) < bodyLen+LEN_SIZE { // 数据不完整,只有部分消息到达
		return nil, 0
	}
	n := int(bodyLen + LEN_SIZE)
	body := buf[LEN_SIZE:n] // 只取本次的数据
	msg := Message{}
	msg.Type = body[0]
	msg.SerialNumber = binary.BigEndian.Uint64(body[TYPE_SIZE : SERIAL_NUMBER_SIZE+TYPE_SIZE])
	uriLen := uint8(body[SERIAL_NUMBER_SIZE+TYPE_SIZE]) // [9]
	msg.Uri = string(body[SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE : SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE+uriLen])
	msg.Data = body[SERIAL_NUMBER_SIZE+TYPE_SIZE+URI_LENGTH_SIZE+uriLen:]
	return msg, n
}

func (messageHandler *LPMessageHandler) MessageReceived(connHandler *ConnHandler, msg interface{}) {
	message := msg.(Message)
	switch message.Type {
	case TYPE_CONNECT:
		go func() {
			log.Println("received connect message:", message.Uri, "=>", string(message.Data))
			addr := string(message.Data)
			realServerMessageHandler := &RealServerMessageHandler{LpConnHandler: connHandler, ConnPool: messageHandler.connPool, UserId: message.Uri, ClientKey: messageHandler.clientKey}
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				log.Println("connect realserver failed", err)
				realServerMessageHandler.ConnFailed()
			} else {
				connHandler := &ConnHandler{}
				connHandler.conn = conn
				connHandler.Listen(conn, realServerMessageHandler)
			}
		}()
	case P_TYPE_TRANSFER:
		if connHandler.NextConn != nil {
			connHandler.NextConn.Write(message.Data)
		}
	case TYPE_DISCONNECT:
		if connHandler.NextConn != nil {
			connHandler.NextConn.NextConn = nil
			connHandler.NextConn.conn.Close()
			connHandler.NextConn = nil
		}
		if messageHandler.clientKey == "" {
			messageHandler.connPool.Return(connHandler)
		}
	}
}

func (messageHandler *LPMessageHandler) ConnSuccess(connHandler *ConnHandler) {
	log.Println("connSuccess, clientkey:", messageHandler.clientKey)
	if messageHandler.clientKey != "" {
		msg := Message{Type: C_TYPE_AUTH}
		msg.Uri = messageHandler.clientKey
		connHandler.Write(msg)
	}
}

func (messageHandler *LPMessageHandler) ConnError(connHandler *ConnHandler) {
	log.Println("connError:", connHandler)
	if messageHandler.die != nil {
		close(messageHandler.die)
	}

	if connHandler.NextConn != nil {
		connHandler.NextConn.NextConn = nil
		connHandler.NextConn.conn.Close()
		connHandler.NextConn = nil
	}

	connHandler.messageHandler = nil
	messageHandler.connHandler = nil
	time.Sleep(time.Second * 3)
}

func (messageHandler *LPMessageHandler) startHeartbeat() {
	log.Println("start heartbeat:", messageHandler.connHandler)
	messageHandler.die = make(chan struct{})
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("run time panic: %v", err)
				debug.PrintStack()
			}
		}()
		for {
			select {
			case <-time.After(time.Second * HEARTBEAT_INTERVAL):
				if time.Now().Unix()-messageHandler.connHandler.ReadTime >= 2*HEARTBEAT_INTERVAL {
					log.Println("proxy connection timeout:", messageHandler.connHandler, time.Now().Unix()-messageHandler.connHandler.ReadTime)
					messageHandler.connHandler.conn.Close()
					return
				}
				msg := Message{Type: TYPE_HEARTBEAT}
				messageHandler.connHandler.Write(msg)
			case <-messageHandler.die:
				return
			}
		}
	}()
}

func (pooler *ProxyConnPooler) Create(pool *ConnHandlerPool) (*ConnHandler, error) {
	var conn net.Conn
	var err error
	if pooler.conf != nil {
		conn, err = tls.Dial("tcp", pooler.addr, pooler.conf)
	} else {
		conn, err = net.Dial("tcp", pooler.addr)
	}

	if err != nil {
		log.Println("Error dialing", err.Error())
		return nil, err
	} else {
		messageHandler := LPMessageHandler{connPool: pool}
		connHandler := &ConnHandler{}
		connHandler.Active = true
		connHandler.conn = conn
		connHandler.messageHandler = interface{}(&messageHandler).(MessageHandler)
		messageHandler.connHandler = connHandler
		messageHandler.startHeartbeat()
		go func() {
			connHandler.Listen(conn, &messageHandler)
		}()
		return connHandler, nil
	}
}

func (pooler *ProxyConnPooler) Remove(conn *ConnHandler) {
	conn.conn.Close()
}

func (pooler *ProxyConnPooler) IsActive(conn *ConnHandler) bool {
	return conn.Active
}

func register() ListenerConfig {
	for {
		registerResponse, err := httpClient.Register()
		if err != nil {
			//log.Println("Register failed...", err)
			time.Sleep(2000 * time.Millisecond)
			continue
		}
		listenerConfig := ListenerConfig{registerResponse.Ip, registerResponse.Port, httpClient.clientKey, registerResponse.SslPort}
		//log.Println("Get listenner info successfully:", listenerConfig)
		return listenerConfig
	}
}

func addDefaultService() {
	for k, v := range proxyConfig.DefaultService {
		err := httpClient.AddPort(k, v)
		if err != nil {
			log.Printf("Add default service: %v failed", err)
		}
	}
}

func init() {
	loadConfig()
	setLogger()
	httpClient = NewHttpClient(proxyConfig.ControllerAddress, TOKEN, proxyConfig.ClientKey)
}

func loadConfig() {
	CONFIG_PATH := os.Getenv("CLOUDCHEF_PROXY_CONFIG_PATH")
	log.Println(CONFIG_PATH)
	yamlFile, err := ioutil.ReadFile(CONFIG_PATH)
	if err != nil {
		log.Println("Config not exists:", err)
		syscall.Exit(1)
	}
	_ = yaml.Unmarshal(yamlFile, &proxyConfig)
	if err != nil {
		log.Println("Parse config failed:", err)
		syscall.Exit(1)
	}
}

func setLogger() {
	logFile, err := os.OpenFile(proxyConfig.LogPath+"cloudchef-proxy.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(mw)
}
