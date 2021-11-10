package invest

import (
	"context"
	pb "goinvest/gen/proto/go/invest/v1"
)

// ProviderID assert for provider id
type ProviderID uint64

const (
	// ProviderTinkoff tinkoff provider
	ProviderTinkoff ProviderID = 1
)

// Uint32 return uint32 for provider id
func (p ProviderID) Uint32() uint32 { return uint32(p) }

type Provider interface {
	// Portfolio retrieves portfolio info
	Portfolio(ctx context.Context, request *pb.PortfolioRequest) (*pb.PortfolioResponse, error)
	// Accounts retrieves portfolio info
	Accounts(ctx context.Context, request *pb.AccountsRequest) (*pb.AccountsResponse, error)
}

// ProvidersConfig config for providers
type ProvidersConfig struct {
	Tinkoff struct {
		TokenSandbox string `yaml:"tokenSandbox"`
		Token        string `yaml:"token"`
		Rps          int    `yaml:"rpc"`
	} `yaml:"google"`
}
