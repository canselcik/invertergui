package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hpdvanwyk/invertergui/mk2if"
	"github.com/mikepb/go-serial"
)

// Basic CLI to serve as example lib usage
func main() {
	//Info = log.New()

	tcp := flag.Bool("tcp", false, "Use TCP instead of TTY")
	ip := flag.String("ip", "localhost:8139", "IP to connect when using tcp connection.")
	dev := flag.String("dev", "/dev/ttyUSB0", "TTY device to use.")
	flag.Parse()

	var p io.ReadWriteCloser
	var err error
	var tcpAddr *net.TCPAddr

	if *tcp {
		tcpAddr, err = net.ResolveTCPAddr("tcp", *ip)
		if err != nil {
			panic(err)
		}
		p, err = net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			panic(err)
		}
	} else {
		options := serial.RawOptions
		options.BitRate = 2400
		options.Mode = serial.MODE_READ_WRITE
		p, err = options.Open(*dev)
		if err != nil {
			panic(err)
		}
	}
	defer p.Close()
	mk2, err := mk2if.NewMk2Connection(p)
	if err != nil {
		panic(err)
	}
	defer mk2.Close()

	c := mk2.C()
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, os.Interrupt)
mainloop:
	for {
		select {
		case tmp := <-c:
			if tmp.Valid {
				PrintInfo(tmp)
			}
		case <-sigterm:
			break mainloop
		}
	}
	log.Printf("Closing connection")
}

func PrintInfo(info *mk2if.Mk2Info) {
	out := fmt.Sprintf("Version: %v\n", info.Version)
	out += fmt.Sprintf("Bat Volt: %.2fV Bat Cur: %.2fA \n", info.BatVoltage, info.BatCurrent)
	out += fmt.Sprintf("In Volt: %.2fV In Cur: %.2fA In Freq %.2fHz\n", info.InVoltage, info.InCurrent, info.InFrequency)
	out += fmt.Sprintf("Out Volt: %.2fV Out Cur: %.2fA Out Freq %.2fHz\n", info.OutVoltage, info.OutCurrent, info.OutFrequency)
	out += fmt.Sprintf("In Power %.2fW Out Power %.2fW\n", info.InVoltage*info.InCurrent, info.OutVoltage*info.OutCurrent)
	out += fmt.Sprintf("Charge State: %.2f%%\n", info.ChargeState*100)
	out += "LEDs state:"
	for k, v := range info.LEDs {
		out += fmt.Sprintf(" %s %s", mk2if.LedNames[k], mk2if.StateNames[v])
	}

	out += "\nErrors:"
	for _, v := range info.Errors {
		out += " " + v.Error()
	}
	out += "\n"
	log.Printf("System Info: \n%v", out)
}
