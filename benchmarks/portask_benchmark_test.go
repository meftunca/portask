package benchmarks

// Temiz importlar:
import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"os"
	"testing"
	"time"

	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/network"
	"github.com/meftunca/portask/pkg/serialization"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

func waitForPortaskServer(addr string, timeout time.Duration) error {
	start := time.Now()
	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for server: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

var (
	testServer *network.TCPServer
)

func TestMain(m *testing.M) {
	cfg := config.DefaultConfig()
	memStore := storage.NewInMemoryStorage()
	codecManager, _ := serialization.NewCodecManager(cfg)
	handler := network.NewPortaskProtocolHandler(codecManager, memStore)
	serverCfg := &network.ServerConfig{
		Address:         ":8080",
		Network:         "tcp",
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
	testServer = network.NewTCPServer(serverCfg, handler)
	go testServer.Start(context.TODO())
	if err := waitForPortaskServer("localhost:8080", 10*time.Second); err != nil {
		fmt.Printf("Server did not start: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	testServer.Stop(context.TODO())
	os.Exit(code)
}

func BenchmarkPortaskPublish(b *testing.B) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	cfg := config.DefaultConfig()
	codecManager, _ := serialization.NewCodecManager(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &types.PortaskMessage{
			ID:        types.MessageID(fmt.Sprintf("%d", i)),
			Topic:     "bench",
			Payload:   []byte(fmt.Sprintf("test-message-%d", i)),
			Timestamp: time.Now().UnixNano(),
		}
		encoded, _ := codecManager.Encode(msg)
		// Portask protokol header'ı oluştur
		head := make([]byte, 16)
		binary.BigEndian.PutUint32(head[0:4], network.ProtocolMagic)
		head[4] = network.ProtocolVersion
		head[5] = network.MessageTypePublish
		// Flags (2 byte): 0
		binary.BigEndian.PutUint16(head[6:8], 0)
		// Payload length (4 byte)
		binary.BigEndian.PutUint32(head[8:12], uint32(len(encoded)))
		// CRC32 (4 byte)
		crc := crc32.ChecksumIEEE(encoded)
		binary.BigEndian.PutUint32(head[12:16], crc)
		// Gönder
		conn.Write(head)
		conn.Write(encoded)
	}
	b.StopTimer()
}

func BenchmarkPortaskPublishParallel(b *testing.B) {
	const numClients = 16
	cfg := config.DefaultConfig()
	codecManager, _ := serialization.NewCodecManager(cfg)
	clients := make([]net.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		clients[i] = conn
	}
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	b.ResetTimer()
	b.SetParallelism(numClients)
	b.RunParallel(func(pb *testing.PB) {
		clientID := int(time.Now().UnixNano()) % numClients
		conn := clients[clientID]
		for i := 0; pb.Next(); i++ {
			msg := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("%d-%d", clientID, i)),
				Topic:     "bench",
				Payload:   []byte(fmt.Sprintf("test-message-%d-%d", clientID, i)),
				Timestamp: time.Now().UnixNano(),
			}
			encoded, _ := codecManager.Encode(msg)
			head := make([]byte, 16)
			binary.BigEndian.PutUint32(head[0:4], network.ProtocolMagic)
			head[4] = network.ProtocolVersion
			head[5] = network.MessageTypePublish
			binary.BigEndian.PutUint16(head[6:8], 0)
			binary.BigEndian.PutUint32(head[8:12], uint32(len(encoded)))
			crc := crc32.ChecksumIEEE(encoded)
			binary.BigEndian.PutUint32(head[12:16], crc)
			conn.Write(head)
			conn.Write(encoded)
		}
	})
	b.StopTimer()
}

func BenchmarkPortaskPublishWithConnPool(b *testing.B) {
	const poolSize = 32
	cfg := config.DefaultConfig()
	codecManager, _ := serialization.NewCodecManager(cfg)
	connPool := make(chan net.Conn, poolSize)
	for i := 0; i < poolSize; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		connPool <- conn
	}
	defer func() {
		close(connPool)
		for c := range connPool {
			c.Close()
		}
	}()

	b.ResetTimer()
	b.SetParallelism(poolSize)
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			conn := <-connPool
			msg := &types.PortaskMessage{
				ID:        types.MessageID(fmt.Sprintf("%d", i)),
				Topic:     "bench",
				Payload:   []byte(fmt.Sprintf("test-message-%d", i)),
				Timestamp: time.Now().UnixNano(),
			}
			encoded, _ := codecManager.Encode(msg)
			head := make([]byte, 16)
			binary.BigEndian.PutUint32(head[0:4], network.ProtocolMagic)
			head[4] = network.ProtocolVersion
			head[5] = network.MessageTypePublish
			binary.BigEndian.PutUint16(head[6:8], 0)
			binary.BigEndian.PutUint32(head[8:12], uint32(len(encoded)))
			crc := crc32.ChecksumIEEE(encoded)
			binary.BigEndian.PutUint32(head[12:16], crc)
			conn.Write(head)
			conn.Write(encoded)
			connPool <- conn // Bağlantıyı havuza geri bırak
		}
	})
	b.StopTimer()
}
