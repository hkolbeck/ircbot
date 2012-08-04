//Copyright 2011 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package ircbot

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"runtime"
	"time"
)

const (
	//Times in nanoseconds
	ReconnectDelay    = 5e9
	KeepAliveInterval = 12e9
	PingTimeout       = 5e9
	ReadTimeout       = 5e9

	CommBufferSize = 16
	nickserv       = `NickServ`
)

type Network struct {
	conn       net.Conn      //Connection to the irc server 
	connIn     *bufio.Reader //Wraps conn's Read/Write operations
	In         chan *Message //Channel containing messages the bot recieves
	Out        chan *Message //Channel containing messages to be sent
	disconnect chan int64    //Internal channel to signal a keepalive failure
	keepalive  chan int      //The bot's PONG command should send a value down this channel
	running    bool          //Flag polled by goroutines to determine if they should continue running
}

func Dial(server string, port int, nick, pass, domain string, ssl bool) (*Network, error) {
	var tcpConn *net.TCPConn
	var conn net.Conn
	var err error
	var network *Network

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%v", server, port))
	if err != nil {
		goto Error
	}

	if tcpConn, err = net.DialTCP("tcp", nil, addr); err != nil {
		goto Error
	}

	if err = tcpConn.SetKeepAlive(true); err != nil {
		goto Error
	}

	if err = tcpConn.SetReadTimeout(ReadTimeout); err != nil {
		goto Error
	}

	if ssl {
		conn = tls.Client(tcpConn, nil) //nil config should work for all cases
	} else {
		conn = tcpConn
	}

	network = &Network{
		conn:       conn,
		connIn:     bufio.NewReader(conn),
		In:         make(chan *Message, CommBufferSize),
		Out:        make(chan *Message, CommBufferSize),
		disconnect: make(chan int64),
		keepalive:  make(chan int),
		running:    true,
	}

	go network.listen()
	go network.speak()
	go network.keepAlive(nick)

	return network, nil

Error:
	return nil, err
}

func (self *Network) HangUp() {
	self.running = false
}

func (self *Network) keepAlive(nick string) {
	tick := time.NewTicker(KeepAliveInterval)
	defer tick.Stop()

	for self.running {
		<-tick.C
		self.Out <- &Message{
			Command: "PING",
			Args:    []string{nick},
		}
		timeout := time.After(PingTimeout)
		select {
		case <-timeout:
			self.disconnect <- 1
		case <-self.keepalive:
			continue
		}
	}
}

func (self *Network) listen() {
	for self.running {
		msg, _, err := self.connIn.ReadLine()
		if err != nil {
			//TODO: Log failure
			//During disconnection, this could spin, make sure reconnect runs
			runtime.Gosched()
			continue
		}
		m := Decode(msg)
		if m.Command != "PONG" && m.Command != "PING" {
			fmt.Println("[>] " + string(msg))
		}

		self.In <- m
	}
}

func (self *Network) speak() {

	for self.running {
		msg := <-self.Out
		_, err := self.conn.Write(msg.Encode())

		//If write fails, keep trying
		for err != nil {
			//During disconnection, this could spin, make sure reconnect runs
			runtime.Gosched()
			_, err = self.conn.Write(msg.Encode())
		}

		if msg.Command != "PONG" && msg.Command != "PING" {
			fmt.Println("[<] " + msg.String())
		}
	}
}
