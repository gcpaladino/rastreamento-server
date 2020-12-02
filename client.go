package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	message       = "*ET,TESTEBROW,HB,A,14091D,14261F,80D1D404,81AF2A84,1E78,7D00,40800000,20,100,00,0000002E,663,1229250520,0000000000,0000,13.80,11#"
	StopCharacter = "\r\n\r\n"
)

func SocketClient(ip string, port int) {
	addr := strings.Join([]string{ip, strconv.Itoa(port)}, ":")
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}

	defer conn.Close()

	conn.Write([]byte(message))
	conn.Write([]byte(StopCharacter))
	log.Printf("Send: %s", message)

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)
	log.Printf("Receive: %s", buff[:n])

}

func main() {

	var (
		ip   = "https://prioritaria-e3-4tnp3vknya-rj.a.run.app"
		port = 7098
	)

	SocketClient(ip, port)

}
