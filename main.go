package main

import (
	"fmt"
	natsserver "github.com/dyammarcano/embeddedNats/nats-server"
	"os"
	"runtime"
)

const (
	serverName = "ConectaNatsServer"
)

func main() {
	ns, err := natsserver.NewNatsServer(serverName)
	if err != nil {
		panic(err)
	}

	if err = ns.CheckAndStart(); err != nil {
		panic(err)
	}

	client, err := ns.GetClient()
	if err != nil {
		panic(err)
	}

	pid := os.Getpid()

	fmt.Printf("pid: %d, status: %s, server: %s", pid, client.Status(), client.ConnectedAddr())

	runtime.Goexit()
}
