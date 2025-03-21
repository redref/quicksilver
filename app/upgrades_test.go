package app

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ingenuity-build/quicksilver/utils"
	icskeeper "github.com/ingenuity-build/quicksilver/x/interchainstaking/keeper"
	tokenfactorytypes "github.com/ingenuity-build/quicksilver/x/tokenfactory/types"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibctesting "github.com/cosmos/ibc-go/v5/testing"
	"github.com/stretchr/testify/suite"

	icstypes "github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
)

func init() {
	ibctesting.DefaultTestingAppInit = SetupTestingApp
}

// TestKeeperTestSuite runs all the tests within this package.
func TestAppTestSuite(t *testing.T) {
	suite.Run(t, new(AppTestSuite))
}

func newQuicksilverPath(chainA, chainB *ibctesting.TestChain) *ibctesting.Path {
	path := ibctesting.NewPath(chainA, chainB)
	path.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	path.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort

	return path
}

type AppTestSuite struct {
	suite.Suite

	coordinator *ibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA *ibctesting.TestChain
	chainB *ibctesting.TestChain

	path *ibctesting.Path
}

func (s *AppTestSuite) GetQuicksilverApp(chain *ibctesting.TestChain) *Quicksilver {
	app, ok := chain.App.(*Quicksilver)
	if !ok {
		panic("not quicksilver app")
	}

	return app
}

// SetupTest creates a coordinator with 2 test chains.
func (suite *AppTestSuite) SetupTest() {
	suite.coordinator = ibctesting.NewCoordinator(suite.T(), 2)         // initializes 2 test chains
	suite.chainA = suite.coordinator.GetChain(ibctesting.GetChainID(1)) // convenience and readability
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(2)) // convenience and readability

	suite.path = newQuicksilverPath(suite.chainA, suite.chainB)
	suite.coordinator.SetupConnections(suite.path)

	suite.coordinator.CurrentTime = time.Now().UTC()
	suite.coordinator.UpdateTime()

	suite.initTestZone()
}

func (suite *AppTestSuite) initTestZone() {
	// test zone
	zone := icstypes.Zone{
		ConnectionId:    suite.path.EndpointA.ConnectionID,
		ChainId:         suite.chainB.ChainID,
		AccountPrefix:   "bcosmos",
		LocalDenom:      "uqatom",
		BaseDenom:       "uatom",
		MultiSend:       false,
		LiquidityModule: true,
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetZone(suite.chainA.GetContext(), &zone)

	// cosmos zone
	zone = icstypes.Zone{
		ConnectionId:    "connection-77001",
		ChainId:         "cosmoshub-4",
		AccountPrefix:   "cosmos",
		LocalDenom:      "uqatom",
		BaseDenom:       "uatom",
		MultiSend:       false,
		LiquidityModule: false,
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetZone(suite.chainA.GetContext(), &zone)

	// osmosis zone
	zone = icstypes.Zone{
		ConnectionId:    "connection-77002",
		ChainId:         "osmosis-1",
		AccountPrefix:   "osmo",
		LocalDenom:      "uqosmo",
		BaseDenom:       "uosmo",
		MultiSend:       false,
		LiquidityModule: true,
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetZone(suite.chainA.GetContext(), &zone)
	// uni-5 zone
	zone = icstypes.Zone{
		ConnectionId:    "connection-77003",
		ChainId:         "uni-5",
		AccountPrefix:   "juno",
		LocalDenom:      "uqjunox",
		BaseDenom:       "ujunox",
		MultiSend:       false,
		LiquidityModule: true,
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetZone(suite.chainA.GetContext(), &zone)

	reciept := icstypes.Receipt{
		ChainId: "uni-5",
		Sender:  utils.GenerateAccAddressForTest().String(),
		Txhash:  "TestDeposit01",
		Amount: sdk.NewCoins(
			sdk.NewCoin(
				"ujunox",
				sdk.NewIntFromUint64(2000000), // 20% deposit
			),
		),
	}

	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetReceipt(suite.chainA.GetContext(), reciept)

	ubRecord := icstypes.UnbondingRecord{
		ChainId:       "uni-5",
		EpochNumber:   1,
		Validator:     "junovaloper185hgkqs8q8ysnc8cvkgd8j2knnq2m0ah6ae73gntv9ampgwpmrxqlfzywn",
		RelatedTxhash: []string{"ABC012"},
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetUnbondingRecord(suite.chainA.GetContext(), ubRecord)

	rdRecord := icstypes.RedelegationRecord{
		ChainId:        "uni-5",
		EpochNumber:    1,
		Source:         "junovaloper185hgkqs8q8ysnc8cvkgd8j2knnq2m0ah6ae73gntv9ampgwpmrxqlfzywn",
		Destination:    "junovaloper1z89utvygweg5l56fsk8ak7t6hh88fd0aa9ywed",
		Amount:         3000000,
		CompletionTime: time.Time(suite.chainA.GetContext().BlockTime().Add(time.Hour)),
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetRedelegationRecord(suite.chainA.GetContext(), rdRecord)

	delRecord := icstypes.Delegation{
		Amount:            sdk.NewCoin(zone.BaseDenom, sdk.NewInt(17000)),
		DelegationAddress: "juno1z89utvygweg5l56fsk8ak7t6hh88fd0azcjpz5",
		Height:            10,
		ValidatorAddress:  "junovaloper185hgkqs8q8ysnc8cvkgd8j2knnq2m0ah6ae73gntv9ampgwpmrxqlfzywn",
	}

	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetDelegation(suite.chainA.GetContext(), &zone, delRecord)

	wRecord := icstypes.WithdrawalRecord{
		ChainId:   "uni-5",
		Delegator: utils.GenerateAccAddressForTest().String(),
		Distribution: []*icstypes.Distribution{
			{Valoper: "junovaloper185hgkqs8q8ysnc8cvkgd8j2knnq2m0ah6ae73gntv9ampgwpmrxqlfzywn", Amount: 1000000},
			{Valoper: "junovaloper1z89utvygweg5l56fsk8ak7t6hh88fd0aa9ywed", Amount: 1000000},
		},
		Recipient:  "juno1z89utvygweg5l56fsk8ak7t6hh88fd0azcjpz5",
		Amount:     sdk.NewCoins(sdk.NewCoin("ujunox", sdk.NewInt(4000000))),
		BurnAmount: sdk.NewCoin("ujunox", sdk.NewInt(4000000)),
		Txhash:     "7C8B95EEE82CB63771E02EBEB05E6A80076D70B2E0A1C457F1FD1A0EF2EA961D",
		Status:     icskeeper.WithdrawStatusQueued,
	}
	suite.GetQuicksilverApp(suite.chainA).InterchainstakingKeeper.SetWithdrawalRecord(suite.chainA.GetContext(), wRecord)

	err := suite.GetQuicksilverApp(suite.chainA).BankKeeper.MintCoins(suite.chainA.GetContext(), tokenfactorytypes.ModuleName, sdk.NewCoins(sdk.NewCoin("uqjunox", sdkmath.NewInt(202000000))))
	if err != nil {
		return
	}
	addr1, err := AccAddressFromBech32("quick17v9kk34km3w6hdjs2sn5s5qjdu2zrm0m3rgtmq", "quick")
	if err != nil {
		return
	}
	addr2, err := AccAddressFromBech32("quick16x03wcp37kx5e8ehckjxvwcgk9j0cqnhcccnty", "quick")
	if err != nil {
		return
	}

	err = suite.GetQuicksilverApp(suite.chainA).BankKeeper.SendCoinsFromModuleToAccount(suite.chainA.GetContext(), tokenfactorytypes.ModuleName, addr1, sdk.NewCoins(sdk.NewCoin("uqjunox", sdkmath.NewInt(1600000))))
	if err != nil {
		return
	}
	err = suite.GetQuicksilverApp(suite.chainA).BankKeeper.SendCoinsFromModuleToAccount(suite.chainA.GetContext(), tokenfactorytypes.ModuleName, addr2, sdk.NewCoins(sdk.NewCoin("uqjunox", sdkmath.NewInt(200000000))))
	if err != nil {
		return
	}
	err = suite.GetQuicksilverApp(suite.chainA).BankKeeper.SendCoinsFromModuleToModule(suite.chainA.GetContext(), tokenfactorytypes.ModuleName, icstypes.EscrowModuleAccount, sdk.NewCoins(sdk.NewCoin("uqjunox", sdkmath.NewInt(400000))))
	if err != nil {
		return
	}

}

func (s *AppTestSuite) TestV010400UpgradeHandler() {
	app := s.GetQuicksilverApp(s.chainA)
	handler := v010400UpgradeHandler(app)
	ctx := s.chainA.GetContext()
	_, err := handler(ctx, types.Plan{}, app.mm.GetVersionMap())
	s.Require().NoError(err)

	osmosis, found := app.InterchainstakingKeeper.GetZone(ctx, "osmosis-1")
	s.Require().True(found)
	s.Require().Equal(int64(6), osmosis.Decimals)
	s.Require().Equal("osmo", osmosis.AccountPrefix)
	s.Require().Equal("connection-77002", osmosis.ConnectionId)
	s.Require().False(osmosis.UnbondingEnabled)
	s.Require().False(osmosis.ReturnToSender)
	s.Require().True(osmosis.LiquidityModule)

	cosmos, found := app.InterchainstakingKeeper.GetZone(ctx, "cosmoshub-4")
	s.Require().True(found)
	s.Require().Equal(int64(6), cosmos.Decimals)
	s.Require().Equal("uatom", cosmos.BaseDenom)
	s.Require().Equal("uqatom", cosmos.LocalDenom)
	s.Require().False(cosmos.UnbondingEnabled)
	s.Require().False(cosmos.ReturnToSender)
	s.Require().False(cosmos.LiquidityModule)

	chainb, found := app.InterchainstakingKeeper.GetZone(ctx, s.chainB.ChainID)
	s.Require().True(found)
	s.Require().Equal(int64(6), chainb.Decimals)
	s.Require().False(chainb.UnbondingEnabled)
	s.Require().False(chainb.ReturnToSender)
	s.Require().True(chainb.LiquidityModule)

	juno, found := app.InterchainstakingKeeper.GetZone(ctx, "uni-5")
	s.Require().False(found)

	reciepts := app.InterchainstakingKeeper.AllReceipts(ctx)
	s.Require().Equal(0, len(reciepts))

	unbondings := app.InterchainstakingKeeper.AllZoneUnbondingRecords(ctx, "uni-5")
	s.Require().Equal(0, len(unbondings))

	redelegations := app.InterchainstakingKeeper.ZoneRedelegationRecords(ctx, "uni-5")
	s.Require().Equal(0, len(redelegations))

	delegations := app.InterchainstakingKeeper.GetAllDelegations(ctx, &juno)
	s.Require().Equal(0, len(delegations))

	perfDelegations := app.InterchainstakingKeeper.GetAllPerformanceDelegations(ctx, &juno)
	s.Require().Equal(0, len(perfDelegations))

	_, found = app.InterchainstakingKeeper.GetWithdrawalRecord(ctx, "uni-5", "7C8B95EEE82CB63771E02EBEB05E6A80076D70B2E0A1C457F1FD1A0EF2EA961D", icskeeper.WithdrawStatusQueued)
	s.Require().False(found)

}
