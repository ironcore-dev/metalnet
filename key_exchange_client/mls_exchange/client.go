package key_exchange_client

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	pb "github.com/ironcore-dev/metalnet/key_exchange_client/mls_exchange/proto"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type KeyExchangeClient struct {
	client pb.AgentServiceClient
	conn   *grpc.ClientConn
	isInit bool
}

func getLocalPrefix() (string, error) {
	// Check if the PREFIX_ADDRESS environment variable is set
	if prefixAddr := os.Getenv("PREFIX_ADDRESS"); prefixAddr != "" {
		if prefixAddr == "random" {
			var ip [16]byte

			// Read 8 random bytes for the 64-bit network portion.
			if _, err := rand.Read(ip[:8]); err != nil {
				return "", err
			}

			// Create the IP, attach the /64 mask, and convert directly to a string
			addr := netip.AddrFrom16(ip)
			prefix := netip.PrefixFrom(addr, 64)

			return prefix.String(), nil
		}
		return prefixAddr, nil
	}

	iface, err := net.InterfaceByName("lo")
	if err != nil {
		return "", fmt.Errorf("could not find 'lo' interface: %v", err)
	}

	// Fetch all addresses assigned to this interface
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("could not fetch addresses: %v", err)
	}

	// Iterate through addresses to find the IPv6 one
	for _, addr := range addrs {
		// addr.String() returns a CIDR string (e.g., "::1/128" or "127.0.0.1/8")
		prefix, err := netip.ParsePrefix(addr.String())
		if err != nil {
			continue // Skip unparseable addresses
		}

		ip := prefix.Addr()

		// Check if it is a pure IPv6 address (ignoring IPv4-mapped IPv6 like ::ffff:127.0.0.1)
		if ip.Is6() && !ip.Is4In6() {
			return ip.String(), nil // use prefix.String() if you also want the /128 subnet mask
		}
	}

	return "", fmt.Errorf("no IPv6 address found on 'lo'")
}

func InitKeyExchangeConnection(agentAddress string) *KeyExchangeClient {
	prefix, err := getLocalPrefix()
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("Error: %s", err)
	}

	logrus.Infof("[%s] agentAddress: %s", hostname, agentAddress)
	logrus.Infof("[%s] ip: %s", hostname, prefix)

	agentClient, err := NewKeyExchangeClient(agentAddress)
	if err != nil {
		logrus.Fatalf("Failed to create agent client: %v", err)
	}

	var initErr error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		initErr = agentClient.Init(context.Background(), hostname, prefix)
		if initErr == nil {
			break
		}
		logrus.Infof("[%s] Agent not ready yet, retrying in 2 seconds... (%d/%d)", hostname, i+1, maxRetries)
		time.Sleep(2 * time.Second)
	}

	if initErr != nil {
		logrus.Fatalf("Failed to init agent after retries: %v", initErr)
	}

	return agentClient
}

func NewKeyExchangeClient(targetURL string) (*KeyExchangeClient, error) {
	conn, err := grpc.NewClient(targetURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent at %s: %v", targetURL, err)
	}

	return &KeyExchangeClient{
		client: pb.NewAgentServiceClient(conn),
		conn:   conn,
		isInit: true,
	}, nil
}

func (kec *KeyExchangeClient) Init(ctx context.Context, clientName, clientPrefix string) error {
	_, err := kec.client.Init(ctx, &pb.AgentInitReq{
		ClientName:   clientName,
		ClientPrefix: clientPrefix,
	})
	return err
}

func (kec *KeyExchangeClient) Subscribe(ctx context.Context, vni uint32) error {
	if kec.isInit == false {
		logrus.Error("Network encryption was requested, but key-exchange was not configured.")
		return nil
	}
	_, err := kec.client.Subscribe(ctx, &pb.AgentSubscribeReq{Vni: vni})
	return err
}

func (kec *KeyExchangeClient) Unsubscribe(ctx context.Context, vni uint32) error {
	if kec.isInit == false {
		return nil
	}
	_, err := kec.client.Unsubscribe(ctx, &pb.AgentUnsubscribeReq{Vni: vni})
	return err
}

func (kec *KeyExchangeClient) IsKeyReady(ctx context.Context, vni, timeout uint32) (bool, error) {
	if kec.isInit == false {
		return true, nil
	}
	for range timeout {
		isReadyRes, err := kec.client.IsKeyReady(ctx, &pb.AgentKeyReadyReq{Vni: vni})
		if err != nil {
			return false, err
		}

		if isReadyRes.IsReady {
			return true, nil
		}

		time.Sleep(1 * time.Second)
	}

	return false, fmt.Errorf("Timeout while requesting key")
}

func (kec *KeyExchangeClient) Close() {
	if kec.isInit == false {
		return
	}
	if kec.conn != nil {
		kec.conn.Close()
	}
}
