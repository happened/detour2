package main

import (
	"flag"
	"fmt"
	"os"

	"detour/local"
	"detour/server"
)

var (
	password string
	listen   string
	remote   string
	proto    string
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("run with server/local subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		ser := flag.NewFlagSet("server", flag.ExitOnError)
		ser.StringVar(&password, "p", "password", "password for authentication")
		ser.StringVar(&listen, "l", "tcp://0.0.0.0:3811", "address to listen on")
		ser.Parse(os.Args[2:])

		s := server.NewServer(listen)
		s.RunServer()
	case "local":
		cli := flag.NewFlagSet("local", flag.ExitOnError)
		cli.StringVar(&remote, "r", "ws://0.0.0.0:3811/ws", "remote server(s) to connect, seperated by comma")
		cli.StringVar(&password, "p", "password", "password for authentication")
		cli.StringVar(&listen, "l", "tcp://0.0.0.0:3810", "address to listen on")
		cli.StringVar(&proto, "t", "socks5", "target protocol to use")

		cli.Parse(os.Args[2:])
		c := local.NewLocal(listen, remote, proto, password)
		c.RunLocal()
	default:
		fmt.Println("only server/local subcommands are allowed")
		os.Exit(1)
	}
}
