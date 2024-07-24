package nats_server

import (
	"context"
	"errors"
	"fmt"
	"github.com/dyammarcano/embeddedNats/lockedfile"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"os"
	"path/filepath"
	"time"
)

var storePath = filepath.Join(os.TempDir(), "store")

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
	}

	nc, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	return &NatsServer{ns: nc, ctx: ctx}, nil
}

func (n *NatsServer) CheckAndStart() error {
	for {
		if tryBecomeLeader() {
			n.ns.Start()

			if !n.ns.ReadyForConnections(5 * time.Second) {
				return errors.New("not ready for connection")
			}

			go func() {
				<-n.ctx.Done()
				n.ns.Shutdown()
			}()
		}

		<-time.After(1 * time.Second) // Give some time to be sure that the server is ready

		var err error
		n.nc, err = nats.Connect(n.ns.ClientURL())
		if err == nil {
			break
		}

		fmt.Println("Failed to connect to NATS server, retrying leader election...")
		time.Sleep(5 * time.Second) // Wait before retrying
	}

	return nil
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

func tryBecomeLeader() bool {
	lockFilePath := filepath.Join(storePath, "leader.lock")

	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.MkdirAll(storePath, 0755)
		}
	}

	if _, err := os.Stat(lockFilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return createLeaderLock(lockFilePath)
		}
	}

	return checkLeaderLock(lockFilePath)
}

func createLeaderLock(lockFilePath string) bool {
	file, err := lockedfile.OpenFile(lockFilePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return false
	}

	if _, err = file.WriteString(fmt.Sprintf("pid:%d", os.Getpid())); err != nil {
		file.Close()
		return false
	}

	return true
}

func checkLeaderLock(lockFilePath string) bool {
	if err := os.Remove(lockFilePath); err != nil {
		return false
	}

	return createLeaderLock(lockFilePath)
}
