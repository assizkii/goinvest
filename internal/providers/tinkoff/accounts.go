package tinkoff

import (
	"context"
	"fmt"
	sdk "github.com/TinkoffCreditSystems/invest-openapi-go-sdk"
	pb "goinvest/gen/proto/go/invest/v1"
)

func (p providerTinkoff) Accounts(ctx context.Context, req *pb.AccountsRequest) (*pb.AccountsResponse, error) {

	r := acquirePortfolioReq()
	defer releasePortfolioReq(r)

	accountsResponse, err := p.client.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load portfolio provider err: %w", err)
	}

	positions := resultFromProviderAccountResponse(accountsResponse)

	return &pb.AccountsResponse{
		Accounts: positions,
	}, err
}

func resultFromProviderAccountResponse(accountsResponse []sdk.Account) []*pb.Account {
	if len(accountsResponse) == 0 {
		return nil
	}
	accounts := make([]*pb.Account, 0, len(accountsResponse))
	for _, account := range accountsResponse {
		accounts = append(accounts, &pb.Account{
			AccountId:   account.ID,
			AccountType: toPbAccountType(account.Type),
		})
	}
	return accounts
}
