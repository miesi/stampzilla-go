package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func getTemps(buf *bufio.Reader, w io.Writer) {
	fmt.Println("frånluft") // Extract air
	Send(buf, w, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x04, 0x08, 0x00})

	fmt.Println("ute") // Outdoor air
	Send(buf, w, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x04, 0x04, 0xec})

	fmt.Println("avluft") // Exhaust air
	Send(buf, w, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x3b, 0x00, 0x95})

	fmt.Println("tilluft") // Supply air
	Send(buf, w, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x39, 0x02, 0xfa})

	fmt.Println("börvärde") // Set temperature
	Send(buf, w, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x04, 0x00, 0x08})
}

func main() {
	// test read escape 0x1b
	test := bytes.NewBuffer([]byte{0x3d, 0x5, 0x0, 0xf5, 0x1b, 0xe4, 0xaf, 0x41, 0x5, 0x3e})
	buff := bufio.NewReader(test)
	read(buff)
	os.Exit(0)
	data := []byte{0x3d, 0x5, 0x0, 0xff, 0x1b, 0x41, 0x92, 0x3e}
	value := data[3 : len(data)-2]
	if len(value) == 3 {
		value = append(value, 0)
		copy(value[1:], value)
		value[0] = 0x00
	}

	fmt.Println(decode(value))
	os.Exit(0)

	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", "192.168.13.57:26486")
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()
	buf := bufio.NewReader(conn)

	boolPtr := flag.Bool("t", false, "print temperatures")
	flag.Parse()

	if *boolPtr {
		getTemps(buf, conn)
		return
	}

	var a, b, c uint8
	for a = 0; a < 255; a++ {
		for b = 0; b < 255; b++ {
			for c = 0; c < 255; c++ {
				Send(buf, conn, []byte{0xff, 0x1e, 0xc8, 0x04, 0xb6, a, b, c})
			}
		}
	}
}

func compileMsg(b []byte) []byte {
	shouldEscape := []uint8{
		0x3c, // client start
		0x3d, // server response
		0x1b, // escape char
		0x3e, // last in msg
	}

	//calculate checksum
	var s byte
	for _, v := range b {
		s ^= v
	}

	escapeChecksum := false
	for _, b := range shouldEscape {
		if s == b {
			escapeChecksum = true
		}
	}

	//[]byte{0x3c, 0xff, 0x1e, 0xc8, 0x04, 0xb6, 0x03, 0x02, 0x0c, 0x96, 0x3e}
	msg := []byte{0x3c}

	for _, a := range b {
		escape := false
		for _, b := range shouldEscape {
			if a == b {
				escape = true
			}
		}

		if escape {
			msg = append(msg, 0x1b, a^0xff) // add escape and bit invert here
		} else {
			msg = append(msg, a)
		}
	}
	if escapeChecksum {
		msg = append(msg, 0x1b, s^0xff, 0x3e) // add escape and bit invert here
		return msg
	}
	msg = append(msg, []byte{s, 0x3e}...)
	return msg
}

func Send(buf *bufio.Reader, w io.Writer, data []byte) {
	sendMsg := compileMsg(data)
	fmt.Printf("send msg: %x\n", sendMsg)
	_, err := w.Write(sendMsg)
	if err != nil {
		logrus.Error(err)
	}

	read(buf)
}

func read(buf *bufio.Reader) {
	msg, err := buf.ReadBytes(0x3e)
	if err != nil {
		log.Fatalln(err)
	}

	// handle escape byte
	indexesToFlip := []int{}
	n := 0
	for _, v := range msg {
		if v == 0x1b {
			indexesToFlip = append(indexesToFlip, n)
			continue
		}
		msg[n] = v
		n++
	}
	msg = msg[:n]
	for _, v := range indexesToFlip {
		msg[v] ^= 0xff
	}

	fmt.Printf("hex:")
	for _, v := range msg {
		fmt.Printf("%#x ", v)
	}
	fmt.Println()

	if msg[1] != 0x05 && msg[2] != 0x00 {
		return
	}

	f, err := decode(msg[3 : len(msg)-2])
	if err != nil {
		return
	}
	fmt.Println("decode float:", f)
}

func decode(data []byte) (float64, error) {
	// this works OK according to protocol example:
	//Extract temperature 21,8
	//0000   3d 05 00 d3 5e ae 41 67 3e
	if len(data) != 4 {
		return 0.0, fmt.Errorf("could not pase float from binary")
	}
	bits := binary.LittleEndian.Uint32(data)
	float := float64(math.Float32frombits(bits))
	return math.Round(float*100) / 100, nil
}