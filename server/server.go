package server

import (
	"context"
	"errors"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrServerNotRunning     = server.ErrServerNotRunning
	ErrServerAlreadyRunning = errors.New("server is already running")
	storePath               = filepath.Join(os.TempDir(), "store")
	pidFilePath             = filepath.Join(storePath, "nats.pid")
)

type NatsServer struct {
	ns  *server.Server
	nc  *nats.Conn
	ctx context.Context
}

func NewNatsServer(serverName string) (*NatsServer, error) {
	return NewNatsServerContext(context.Background(), serverName)
}

func NewNatsServerContext(ctx context.Context, serverName string) (*NatsServer, error) {
	opts := &server.Options{
		Port:               45733,
		ServerName:         serverName,
		JetStream:          true,
		JetStreamMaxMemory: 1 << 30,
		JetStreamMaxStore:  1 << 30,
		StoreDir:           storePath,
		PidFile:            pidFilePath,
	}

	nc, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	return &NatsServer{
		ns:  nc,
		ctx: ctx,
	}, nil
}

// CheckAndStart check if server is running using some network connection, if not then create lock file to prevent multiple instances
// of the server running, then start the server, if server is already running connect to it.
func (n *NatsServer) CheckAndStart() error {
	if n.isRunning() {
		return nil
	}

	n.ns.Start()

	if !n.ns.ReadyForConnections(10 * time.Second) {
		return errors.New("not ready for connection")
	}

	go func() {
		<-n.ctx.Done()
		n.ns.Shutdown()
	}()

	var err error
	n.nc, err = nats.Connect(n.ns.ClientURL())
	if err != nil {
		return err
	}

	return nil
}

func (n *NatsServer) isRunning() bool {
	var err error
	n.nc, err = nats.Connect(n.ns.ClientURL())
	return err == nil
}

func (n *NatsServer) GetClient() (*nats.Conn, error) {
	if n.nc != nil {
		return n.nc, nil
	}

	return nil, errors.New("client not connected")
}

func (n *NatsServer) Close() {
	n.ns.Shutdown()
}

func (n *NatsServer) WaitForShutdown() {
	n.ns.WaitForShutdown()
}
