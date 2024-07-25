package nats

import (
	"context"
	"errors"
	"fmt"
	"github.com/dyammarcano/embeddedNats/embed/lockedfile"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"log/slog"
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

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	return &NatsServer{ns: ns, ctx: ctx}, nil
}

func (n *NatsServer) CheckAndStart() error {
	for {
		if n.tryBecomeLeader() {
			n.ns.Start()

			slog.Info("NATS server is starting...")

			if !n.ns.ReadyForConnections(5 * time.Second) {
				slog.Error("NATS server is not ready for connection")
				return errors.New("not ready for connection")
			}

			slog.Info("NATS server is ready for connection")

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

		slog.Info("Failed to connect to NATS server, retrying leader election...")
		time.Sleep(5 * time.Second) // Wait before retrying
	}

	go n.monitorConnection()

	return nil
}

func (n *NatsServer) GetClient() (*nats.Conn, error) {
	if n.nc == nil || n.nc.Status() != nats.CONNECTED {
		slog.Info("NATS client is not connected, checking server status...")

		if err := n.CheckAndStart(); err != nil {
			return nil, err
		}
	}

	return n.nc, nil
}

func (n *NatsServer) Close() {
	n.ns.Shutdown()
}

func (n *NatsServer) WaitForShutdown() {
	n.ns.WaitForShutdown()
}

func (n *NatsServer) tryBecomeLeader() bool {
	slog.Info("Trying to become leader...")

	lockFilePath := filepath.Join(storePath, "leader.lock")

	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.MkdirAll(storePath, 0755)
		}
	}

	if _, err := os.Stat(lockFilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return n.createLeaderLock(lockFilePath)
		}
	}

	return n.checkLeaderLock(lockFilePath)
}

func (n *NatsServer) createLeaderLock(lockFilePath string) bool {
	slog.Info("Creating leader lock...")

	file, err := lockedfile.OpenFile(lockFilePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return false
	}

	if _, err = file.WriteString(fmt.Sprintf("pid:%d", os.Getpid())); err != nil {
		file.Close()
		return false
	}

	go func() {
		<-n.ctx.Done()
		file.Close()
	}()

	return true
}

func (n *NatsServer) checkLeaderLock(lockFilePath string) bool {
	if err := os.Remove(lockFilePath); err != nil {
		slog.Info("Failed to acquire leader lock...")
		return false
	}

	return n.createLeaderLock(lockFilePath)
}

func (n *NatsServer) monitorConnection() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			n.nc.Close()
			return
		case <-ticker.C:
			if n.nc.Status() != nats.CONNECTED {
				slog.Info("NATS client lost connection, attempting leader election...")
				if err := n.CheckAndStart(); err != nil {
					slog.Error("Failed to re-establish NATS connection:", err)
				}
			}
		}
	}
}
