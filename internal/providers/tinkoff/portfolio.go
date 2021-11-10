package tinkoff

import (
	"context"
	"fmt"
	sdk "github.com/TinkoffCreditSystems/invest-openapi-go-sdk"
	pb "goinvest/gen/proto/go/invest/v1"
	"sync"
)

var (
	portfolioReqPool = sync.Pool{}
)

// acquirePortfolioReq returns an empty geocode req instance from request pool.
//
// The returned Request instance may be passed to release function when it is
// no longer needed. This allows request recycling, reduces GC pressure
// and usually improves performance.
func acquirePortfolioReq() *sdk.Portfolio {
	v := portfolioReqPool.Get()
	if v == nil {
		return &sdk.Portfolio{}
	}
	return v.(*sdk.Portfolio)
}

// releasePortfolioReq returns request acquired via acquirePortfolioReq to request pool.
// It is forbidden accessing order and/or its' members after returning
// it to request pool.
func releasePortfolioReq(portfolio *sdk.Portfolio) {
	resetPortfolio(portfolio)
	portfolioReqPool.Put(portfolio)
}

func (p providerTinkoff) Portfolio(ctx context.Context, req *pb.PortfolioRequest) (*pb.PortfolioResponse, error) {

	r := acquirePortfolioReq()
	defer releasePortfolioReq(r)

	if req.Account == nil {
		return nil, fmt.Errorf("account is nil")
	}

	//register, err := p.sandboxClient.Register(context.Background(), toSdkAccountType(req.Account.AccountType))
	//if err != nil {
	//	return nil, err
	//}

	portfolioResponse, err := p.client.Portfolio(ctx, req.Account.AccountId)
	if err != nil {
		return nil, fmt.Errorf("load portfolio provider err: %w", err)
	}

	positions := resultFromProviderPortfolioResponse(portfolioResponse)

	return &pb.PortfolioResponse{
		Positions: positions,
	}, err
}

func resultFromProviderPortfolioResponse(portfolioResponse sdk.Portfolio) []*pb.Position {
	if len(portfolioResponse.Positions) == 0 {
		return nil
	}
	positions := make([]*pb.Position, 0, len(portfolioResponse.Positions))
	for _, position := range portfolioResponse.Positions {
		positionPb := &pb.Position{
			Figi:           position.FIGI,
			Ticker:         position.Ticker,
			Isin:           position.ISIN,
			InstrumentType: string(position.InstrumentType),
			Balance:        position.Balance,
			Blocked:        position.Blocked,
			ExpectedYield: &pb.Yield{
				Currency: string(position.ExpectedYield.Currency),
				Value:    position.ExpectedYield.Value,
			},
			Lots: int32(position.Lots),
			AveragePositionPrice: &pb.Yield{
				Currency: string(position.AveragePositionPrice.Currency),
				Value:    position.AveragePositionPrice.Value,
			},
			AveragePositionPriceNoNkd: &pb.Yield{
				Currency: string(position.AveragePositionPriceNoNkd.Currency),
				Value:    position.AveragePositionPriceNoNkd.Value,
			},
			Name: position.Name,
		}
		positions = append(positions, positionPb)
	}

	return positions
}

func resetPortfolio(portfolio *sdk.Portfolio) {
	*portfolio = sdk.Portfolio{}
}

func (p providerTinkoff) additionalInfo(*pb.Position) {

}
