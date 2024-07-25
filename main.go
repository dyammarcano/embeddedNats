package main

import (
	"fmt"
	"github.com/dyammarcano/embeddedNats/embed/nats"
	"log/slog"
	"os"
	"runtime"
)

const (
	serverName = "ConectaNatsServer"
)

func main() {
	ns, err := nats.NewNatsServer(serverName)
	if err != nil {
		panic(err)
	}

	client, err := ns.GetClient()
	if err != nil {
		panic(err)
	}

	pid := os.Getpid()

	slog.Info("msg", slog.Int("pid", pid), slog.String("status", fmt.Sprintf("%s", client.Status())), slog.String("server", client.ConnectedAddr()))

	runtime.Goexit()
}
