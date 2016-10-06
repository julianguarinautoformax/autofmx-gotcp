/* ... <== see fragment description ... */

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/autofmx/afmxdec"
	"github.com/autofmx/afmxsrv"
	"io"
	"io/ioutil"
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
type AutofmxPacket struct {
	buff []byte
}

func (epk *AutofmxPacket) Serialize() []byte {
	fmt.Println("Serialize")
	return epk.buff
}
func (epk *AutofmxPacket) GetLength() uint32 {
	return binary.BigEndian.Uint32(epk.buff[0:4])
}
func (epk *AutofmxPacket) GetBody() []byte {
	return epk.buff[4:]
}
func (epk *AutofmxPacket) GetSizeDataBlock() (uint32, []byte, []byte) {

	epkbuffsz := uint32(len(epk.buff))
	sz := epk.GetLength()

	if (sz + 4) == epkbuffsz {
		return sz, epk.buff[4 : 4+sz], nil
	}
	return sz, epk.buff[4 : 4+sz], epk.buff[4+sz:]

}
func NewAutofmxPacket(buff []byte, hasLengthField bool) *AutofmxPacket {

	/* Make a new Autoformax Packer */
	p := &AutofmxPacket{}

	/* Check if it should go with size pre field */
	if hasLengthField {
		p.buff = buff
	} else {

		p.buff = make([]byte, 4+len(buff))
		binary.BigEndian.PutUint32(p.buff[0:4], uint32(len(buff)))
		copy(p.buff[4:], buff)
	}
	return p
}

type AutofmxProtocol struct {
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
func (epl *AutofmxProtocol) ReadPacket(conn *net.TCPConn) (afmxsrv.Packet, error) {

	var sz uint32
	var szByteField []byte = make([]byte, 4)

	// Full 4 byte field.
	fmt.Println("ReadingPacket.")
	if nRead, err := io.ReadFull(conn, szByteField); err != nil {
		return nil, err
	} else {
		fmt.Println("[", nRead, "]:", szByteField)
	}

	// Get from bytefield the sz of the rest of the packet. sz is not to be higher than 16 MEGS.
	if sz = binary.BigEndian.Uint32(szByteField); sz > (1 << 24) {
		fmt.Println("Packet oversized:",sz,1<<24)
		return nil, errors.New("Packet oversized.")
	}
	fmt.Println("Size is:", sz)

	//Create a buffer that fits the size of the rest of the packet.
	buff := make([]byte, sz)

	// Read the rest of the packet.
	if nRead, err := io.ReadFull(conn, buff); err != nil {
		fmt.Println("err:", err, nRead)
		return nil, err
	} else {
		fmt.Println("[", nRead, "]")
	}

	return NewAutofmxPacket(buff, true), nil
}

type Callback struct{}

func (cb *Callback) OnConnect(c *afmxsrv.Conn) bool {
	addr := c.GetRawConn().RemoteAddr()
	c.PutExtraData(addr)

	fmt.Println("OnConnect:", addr)
	return true
}
func (cb *Callback) OnMessage(c *afmxsrv.Conn, p afmxsrv.Packet) bool {

	fmt.Println("OnMessage")

	//(*AutofmxPacket) ----> Notation to cast the type.
	autofmxPacket := p.(*AutofmxPacket)

	//Get data.
	fmt.Println("Parsing json:")
	_, jsonBuffer, pngBuffer := autofmxPacket.GetSizeDataBlock()
	autoformaxImageInfo, _ := afmxdec.ParseAutoFMXImageMetaInfo(jsonBuffer)
	fmt.Println(autoformaxImageInfo.OriginatorHardwareAddress, autoformaxImageInfo.OriginatorHardwareAddressString, autoformaxImageInfo.OriginatorTimeStampUTC, autoformaxImageInfo.OriginatorTimeStampUTCString)

	/* Save PNG */
	filename := os.Getenv("IMAGES_REPOSITORY") + "/" + autoformaxImageInfo.OriginatorHardwareAddressString + "_" + autoformaxImageInfo.OriginatorTimeStampUTCString + ".png"
	err := ioutil.WriteFile(filename, pngBuffer, 0644)
	if err != nil {
		fmt.Println("Problem writing file.")
		/* Notify administrator */
	} else {
		fmt.Println("Save a ", len(pngBuffer), " bytes PNG")
	}
	fmt.Println(filename)
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
	srv := afmxsrv.NewServer(config, &Callback{}, &AutofmxProtocol{})

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
