package tinkoff

import (
	sdk "github.com/TinkoffCreditSystems/invest-openapi-go-sdk"
	pb "goinvest/gen/proto/go/invest/v1"
)

func toSdkAccountType(pbType pb.AccountType) sdk.AccountType {
	switch pbType {
	case pb.AccountType_TYPE_IIS:
		return sdk.AccountTinkoffIIS
	case pb.AccountType_TYPE_BROKER:
		return sdk.AccountTinkoff
	default:
		return sdk.DefaultAccount
	}
}

func toPbAccountType(sdkType sdk.AccountType) pb.AccountType {
	switch sdkType {
	case sdk.AccountTinkoffIIS:
		return pb.AccountType_TYPE_IIS
	case sdk.AccountTinkoff:
		return pb.AccountType_TYPE_BROKER
	default:
		return pb.AccountType_TYPE_UNSPECIFIED
	}
}
