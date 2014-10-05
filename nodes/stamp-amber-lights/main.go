package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"github.com/stampzilla/stampzilla-go/protocol"
	"github.com/stampzilla/stampzilla-go/nodes/basenode"
	"github.com/tarm/goserial"
	"io"
	"bytes"
	"encoding/binary"
	"strconv"
	"math/rand"
	"time"
)

var node *protocol.Node
var c0 *SerialConnection;

var targetColor [4]byte;
var state *State = &State{[]*Device{},make(map[string]*Sensor,0)};


var send chan string = make(chan string)

type SerialConnection struct {
    Name string
    Baud int
	Port io.ReadWriteCloser
}

func init() {
	// Load flags
	var host string
	var port string
	var dev string
	flag.StringVar(&host, "host", "localhost", "Stampzilla server hostname")
	flag.StringVar(&port, "port", "8282", "Stampzilla server port")
	flag.StringVar(&dev, "dev", "/dev/ttyACM0", "Arduino serial port")
	flag.Parse()

	//Setup Config
	basenode.SetConfig(
		&basenode.Config{
			Host: host,
			Port: port})

	//Start communication with the server
	recv := make(chan protocol.Command)
	connectionState := basenode.Connect(send, recv)
	go monitorState(connectionState, send)
	go serverRecv(recv)

	// Setup the serial connection
	c0 = &SerialConnection{Name: dev, Baud: 9600}
}

func main() {
	// Create new node description
	node = protocol.NewNode("stamp-amber-lights")
	node.SetState(state)
	state.Sensors["temp1"] = NewSensor("temp1","Temperature - Bottom level","20C");
	state.Sensors["temp2"] = NewSensor("temp2","Temperature - Top level","30C");
	state.Sensors["press"] = NewSensor("press","Air pressure","1019 hPa");

	// Describe available actions
	node.AddAction("set", "Set", []string{"Devices.Id"})
	node.AddAction("toggle", "Toggle", []string{"Devices.Id"})
	node.AddAction("dim", "Dim", []string{"Devices.Id", "value"})

	// Describe available layouts
	node.AddLayout("1", "switch", "toggle", "Devices", []string{"on"}, "Lights")
	node.AddLayout("2", "slider", "dim", "Devices", []string{"dim"}, "Lights")
	node.AddLayout("3", "color-picker", "dim", "Devices", []string{"color"}, "Lights")
	node.AddLayout("4", "text", "", "Sensors", []string{}, "Sensors")

	state.AddDevice("0","Color",[]string{"color"},"0");
	state.AddDevice("1","Red",[]string{"dim"},"0");
	state.AddDevice("2","Green",[]string{"dim"},"0");
	state.AddDevice("3","Blue",[]string{"dim"},"0");

	c0.connect();

	for {
		select {
			case <- time.After(time.Second):
				state.Sensors["press"].State = strconv.FormatInt(int64(rand.Intn(40) + 980),10) +" hPa"
					d, err := node.JsonEncode()
					if err != nil {
						log.Error(err)
					}
					send <- d
		}
	}
}

func monitorState(connectionState chan int, send chan string) {
	for s := range connectionState {
		switch s {
		case basenode.ConnectionStateConnected:
			d, err := node.JsonEncode()
			if err != nil {
				log.Error(err)
			}
			send <- d
		case basenode.ConnectionStateDisconnected:
		}
	}
}

func serverRecv(recv chan protocol.Command) {

	for d := range recv {
		processCommand(d)
	}

}

func processCommand(cmd protocol.Command) {

	type Cmd struct {
		Cmd uint16
		Arg uint32
	}

	type CmdColor struct {
		Cmd uint16
		Arg [4]byte
	}

	buf := new(bytes.Buffer)

	log.Info(cmd);

	switch cmd.Cmd {
	case "dim":
		value,_ := strconv.ParseInt(cmd.Args[1], 10, 32);

		value *= 255;
		value /= 100;

		switch(cmd.Args[0]) {
			case "1":
				targetColor[0] = byte(value);
			case "2":
				targetColor[1] = byte(value);
			case "3":
				targetColor[2] = byte(value);
		}

		err := binary.Write(buf, binary.BigEndian, &CmdColor{Cmd: 1, Arg: targetColor })
		if err != nil {
			log.Error("binary.Write failed:", err)
		}
	default:
		return;
	}

		n, err := c0.Port.Write(buf.Bytes())
		if err != nil {
			log.Error(err)
		}
		log.Info("Wrote ",n," bytes");
}


func (config *SerialConnection) connect() {

	c := &serial.Config{Name: config.Name, Baud: config.Baud}
	var err error

    config.Port, err = serial.OpenPort(c)
    if err != nil {
		log.Critical(err)
    }

	go func() {
		for {
			  buf := make([]byte, 128)

			  _, err := config.Port.Read(buf)
			  if err != nil {
					  log.Critical(err)
					return
			  }
			  //log.Info("IN: ", string(buf[:n]) )
		}
	}()
}