package keeper

import (
	"github.com/tendermint/tendermint/libs/log"

	"github.com/ingenuity-build/quicksilver/x/mint/types"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	airdroptypes "github.com/ingenuity-build/quicksilver/x/airdrop/types"
	participationrewards "github.com/ingenuity-build/quicksilver/x/participationrewards/types"
)

// Keeper of the mint store.
type Keeper struct {
	cdc              codec.BinaryCodec
	storeKey         storetypes.StoreKey
	paramSpace       paramtypes.Subspace
	accountKeeper    types.AccountKeeper
	bankKeeper       types.BankKeeper
	distrKeeper      types.DistrKeeper
	epochKeeper      types.EpochKeeper
	hooks            types.MintHooks // should probably add a setter for this somewhere
	feeCollectorName string
}

// NewKeeper creates a new mint Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	key storetypes.StoreKey,
	paramSpace paramtypes.Subspace,
	ak types.AccountKeeper,
	bk types.BankKeeper,
	dk types.DistrKeeper,
	epochKeeper types.EpochKeeper,
	feeCollectorName string,
) Keeper {
	// ensure mint module account is set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the mint module account has not been set")
	}

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:              cdc,
		storeKey:         key,
		paramSpace:       paramSpace,
		accountKeeper:    ak,
		bankKeeper:       bk,
		distrKeeper:      dk,
		epochKeeper:      epochKeeper,
		feeCollectorName: feeCollectorName,
	}
}

// _____________________________________________________________________

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// Set the mint hooks.
func (k *Keeper) SetHooks(h types.MintHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set mint hooks twice")
	}

	k.hooks = h

	return k
}

// GetLastReductionEpochNum returns last Reduction epoch number.
func (k Keeper) GetLastReductionEpochNum(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(types.LastReductionEpochKey)
	if b == nil {
		return 0
	}

	return int64(sdk.BigEndianToUint64(b))
}

// SetLastReductionEpochNum set last Reduction epoch number.
func (k Keeper) SetLastReductionEpochNum(ctx sdk.Context, epochNum int64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.LastReductionEpochKey, sdk.Uint64ToBigEndian(uint64(epochNum)))
}

// get the minter.
func (k Keeper) GetMinter(ctx sdk.Context) (minter types.Minter) {
	store := ctx.KVStore(k.storeKey)
	b := store.Get(types.MinterKey)
	if b == nil {
		panic("stored minter should not have been nil")
	}

	k.cdc.MustUnmarshal(b, &minter)
	return
}

// set the minter.
func (k Keeper) SetMinter(ctx sdk.Context, minter types.Minter) {
	store := ctx.KVStore(k.storeKey)
	b := k.cdc.MustMarshal(&minter)
	store.Set(types.MinterKey, b)
}

// _____________________________________________________________________

// GetParams returns the total set of minting parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return params
}

// SetParams sets the total set of minting parameters.
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) {
	k.paramSpace.SetParamSet(ctx, &params)
}

// _____________________________________________________________________

// MintCoins implements an alias call to the underlying supply keeper's
// MintCoins to be used in BeginBlocker.
func (k Keeper) MintCoins(ctx sdk.Context, newCoins sdk.Coins) error {
	if newCoins.Empty() {
		// skip as no coins need to be minted
		return nil
	}

	return k.bankKeeper.MintCoins(ctx, types.ModuleName, newCoins)
}

// GetProportions gets the balance of the `MintedDenom` from minted coins and returns coins according to the `AllocationRatio`.
func (k Keeper) GetProportions(ctx sdk.Context, mintedCoin sdk.Coin, ratio sdk.Dec) sdk.Coin {
	return sdk.NewCoin(mintedCoin.Denom, sdk.NewDecFromInt(mintedCoin.Amount).Mul(ratio).TruncateInt())
}

// DistributeMintedCoins implements distribution of minted coins from mint to external modules.
func (k Keeper) DistributeMintedCoin(ctx sdk.Context, mintedCoin sdk.Coin) error {
	params := k.GetParams(ctx)
	proportions := params.DistributionProportions

	// allocate staking incentives into fee collector account to be moved to on next begin blocker by staking module
	stakingIncentivesCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoin, proportions.Staking))
	err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, k.feeCollectorName, stakingIncentivesCoins)
	if err != nil {
		return err
	}

	// allocate pool allocation ratio to pool-incentives module account account
	poolIncentivesCoins := sdk.NewCoins(k.GetProportions(ctx, mintedCoin, proportions.PoolIncentives))
	// temporary until we have incentives pool sorted :)

	err = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, airdroptypes.ModuleName, poolIncentivesCoins)
	if err != nil {
		return err
	}

	participationRewardCoin := k.GetProportions(ctx, mintedCoin, proportions.ParticipationRewards)
	participationRewardCoins := sdk.NewCoins(participationRewardCoin)
	participationRewardsAddress := k.accountKeeper.GetModuleAddress(participationrewards.ModuleName)
	k.Logger(ctx).Info("participation rewards", "Proportion", proportions.ParticipationRewards, "Coins", participationRewardCoins, "Address", participationRewardsAddress)
	err = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, participationrewards.ModuleName, participationRewardCoins)
	if err != nil {
		return err
	}

	// Take the current balance of the developer rewards pool and remove it from the supply offset
	// We re-introduce the new supply at the end, in order to avoid any rounding discrepancies.
	// developerAccountBalance := k.bankKeeper.GetBalance(ctx, k.accountKeeper.GetModuleAddress(types.DeveloperVestingModuleAcctName), mintedCoin.Denom)
	// k.bankKeeper.AddSupplyOffset(ctx, mintedCoin.Denom, developerAccountBalance.Amount)

	// // Take the new balance of the developer rewards pool and add it back to the supply offset deduction
	// developerAccountBalance = k.bankKeeper.GetBalance(ctx, k.accountKeeper.GetModuleAddress(types.DeveloperVestingModuleAcctName), mintedCoin.Denom)
	// k.bankKeeper.AddSupplyOffset(ctx, mintedCoin.Denom, developerAccountBalance.Amount.Neg())

	// subtract from original provision to ensure no coins left over after the allocations
	communityPoolCoins := sdk.NewCoins(mintedCoin).Sub(stakingIncentivesCoins...).Sub(poolIncentivesCoins...).Sub(participationRewardCoins...)
	err = k.distrKeeper.FundCommunityPool(ctx, communityPoolCoins, k.accountKeeper.GetModuleAddress(types.ModuleName))
	if err != nil {
		return err
	}

	// call a hook after the minting and distribution of new coins
	// check if hooks are set (as this requires a concrete implementation)
	if k.hooks != nil {
		k.hooks.AfterDistributeMintedCoin(ctx, mintedCoin)
	}

	return err
}
