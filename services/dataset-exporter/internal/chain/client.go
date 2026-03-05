package chain

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client connects to a ZERONE chain node via gRPC.
type Client struct {
	conn     *grpc.ClientConn
	endpoint string
}

// Sample mirrors the on-chain knowledge Sample for local use.
type Sample struct {
	ID             string
	Content        string
	SampleType     string
	Domain         string
	QualityTier    string
	QualityScore   int
	NoveltyScore   int
	SourceURI      string
	SourcePlatform string
	OriginalAuthor string
	Language       string
	Tags           []string
	ThreadID       string
	ParentSampleID string
	ThreadPosition int
	BlockHeight    int64
}

// New creates a new chain gRPC client.
func New(endpoint string) (*Client, error) {
	if endpoint == "" {
		endpoint = "localhost:9090"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial chain grpc %s: %w", endpoint, err)
	}

	return &Client{conn: conn, endpoint: endpoint}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Conn returns the underlying gRPC connection for query clients.
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// FetchApprovedSamples fetches all approved samples above a given block height.
// In production this would call the x/knowledge QuerySamplesByStatus RPC.
// For now, we provide the interface and a polling stub.
func (c *Client) FetchApprovedSamples(ctx context.Context, sinceHeight int64) ([]*Sample, int64, error) {
	// TODO: Wire to actual x/knowledge gRPC query once proto client is generated.
	// QueryClient.SamplesByStatus(ctx, &types.QuerySamplesByStatusRequest{
	//     Status: "gold,silver,bronze",
	//     SinceBlockHeight: sinceHeight,
	// })
	log.Printf("chain: polling for approved samples since height %d", sinceHeight)
	return nil, sinceHeight, nil
}

// FetchSampleByID fetches a single sample by its on-chain ID.
func (c *Client) FetchSampleByID(ctx context.Context, id string) (*Sample, error) {
	// TODO: Wire to actual x/knowledge gRPC query.
	// QueryClient.Sample(ctx, &types.QuerySampleRequest{Id: id})
	return nil, fmt.Errorf("sample %s: not implemented — wire to chain gRPC", id)
}

// FetchSamplesByDomain fetches samples filtered by domain.
func (c *Client) FetchSamplesByDomain(ctx context.Context, domain string, minQuality string) ([]*Sample, error) {
	// TODO: Wire to actual gRPC query.
	log.Printf("chain: fetching samples for domain=%s minQuality=%s", domain, minQuality)
	return nil, nil
}

// LatestBlockHeight returns the latest block height from the chain.
func (c *Client) LatestBlockHeight(ctx context.Context) (int64, error) {
	// TODO: Wire to CometBFT status RPC or ABCI info query.
	return 0, fmt.Errorf("latest block height: not implemented — wire to chain RPC")
}
