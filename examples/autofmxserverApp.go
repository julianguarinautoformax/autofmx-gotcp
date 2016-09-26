/* ... <== see fragment description ... */

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/autofmx/afmxsrv"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// //////////////////////////////////////
// Echo custom Packet protocol: how we deal with Packet
// 2 Types and their methods
// The code that follows is there as a learning facility (see Server)
// it should be written in a specific file, part of the package
//
type EchoPacket struct {
	buff []byte
}

func (epk *EchoPacket) Serialize() []byte {
	fmt.Println("Serialize")
	return epk.buff
}
func (epk *EchoPacket) GetLength() uint32 {
	return binary.BigEndian.Uint32(epk.buff[0:4])
}
func (epk *EchoPacket) GetBody() []byte {
	return epk.buff[4:]
}
func NewEchoPacket(buff []byte, hasLengthField bool) *EchoPacket {
	p := &EchoPacket{}
	if hasLengthField {
		p.buff = buff
	} else {
		p.buff = make([]byte, 4+len(buff))
		binary.BigEndian.PutUint32(p.buff[0:4], uint32(len(buff)))
		copy(p.buff[4:], buff)
	}
	return p
}

type RecieveImageProtocol struct {
}

/**
 * @brief      Reads, validates & saves as PNG the info in the packet.\nIn the future this function will only as receiver. Validation should be delivered to another process by means of a channel. That validation must:
 *
 * 1. Identify the packet type.
 * 2. Deliver the packet to a proper management function.
 *
 *
 * @param      conn  The TCP connection.
 *
 * @return     This function returns the size of the packet and the packet itself and a error.
 */
func (epl *RecieveImageProtocol) ReadPacket(conn *net.TCPConn) (int, afmxsrv.Packet, error) {

	var (
		lengthTotalBytes []byte = make([]byte, 4)
	)

	// read lengthTotal, io.ReadFull deals with EOF
	if nRead, err := io.ReadFull(conn, nReadBytes); err != nil {
		return nil, err
	} else {
		fmt.Println("[", nRead, "]:", lengthTotalBytes)
	}

	// Twist the byte-lenght 4 bytes.
	if lengthTotal = binary.BigEndian.Uint32(lengthTotalBytes); lengthTotal > (1 << 20) {
		fmt.Println("Packet Size blow....")
		return nil, errors.New("the size of packet is larger than the limit")
	}

	//Use a Buffer to get the rest......
	buff := make([]byte, 4+lengthTotal)
	copy(buff[0:4], lengthTotalBytes)

	// read body ( buff = lengthTotalBytes + body )
	if nRead, err := io.ReadFull(conn, buff[4:]); err != nil {
		fmt.Println("err:", err, nRead)
		return 0, nil, err
	} else {
		fmt.Println("No error, sized bytes:", nRead)
	}

	return NewEchoPacket(buff, true), nil
}

type Callback struct{}

func (cb *Callback) OnConnect(c *afmxsrv.Conn) bool {
	addr := c.GetRawConn().RemoteAddr()
	c.PutExtraData(addr)

	fmt.Println()

	fmt.Println("OnConnect:", addr)
	return true
}
func (cb *Callback) OnMessage(c *afmxsrv.Conn, p afmxsrv.Packet) bool {
	fmt.Println("hello")
	echoPacket := p.(*EchoPacket)
	message := "OnMessage: received packet length is[%v]\t" +
		"received packet Body is [%v]\n"
	fmt.Printf(message, echoPacket.GetLength())
	//c.AsyncWritePacket(NewEchoPacket(echoPacket.Serialize(), true), time.Second)
	return true

}
func (cb *Callback) OnClose(c *afmxsrv.Conn) {
	fmt.Println("OnClose:", c.GetExtraData())
}
func main() {
	a := runtime.NumCPU()
	fmt.Println("NumCPU", a)
	runtime.GOMAXPROCS(runtime.NumCPU())

	// creates a tcp listener
	tcpAddr, err := net.ResolveTCPAddr("tcp4", ":6868")
	checkError(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err)

	// creates a server
	config := &afmxsrv.Config{
		PacketSendChanLimit:    2000,
		PacketReceiveChanLimit: 2000,
	}
	srv := afmxsrv.NewServer(config, &Callback{}, &EchoProtocol{})

	// starts service
	go srv.Start(listener, time.Second)
	fmt.Println("listening:", listener.Addr())

	// catchs system signal
	chSig := make(chan os.Signal)
	signal.Notify(chSig, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Signal: ", <-chSig)

	// stops service
	srv.Stop()
}
func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
