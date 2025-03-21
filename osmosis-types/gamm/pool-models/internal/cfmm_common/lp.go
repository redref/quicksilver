package cfmm_common

import (
	sdkioerrors "cosmossdk.io/errors"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ingenuity-build/quicksilver/osmosis-types/gamm"

	"github.com/ingenuity-build/quicksilver/osmosis-types/osmoutils"
)

const errMsgFormatSharesLargerThanMax = "%s resulted shares is larger than the max amount of %s"

// CalcExitPool returns how many tokens should come out, when exiting k LP shares against a "standard" CFMM
func CalcExitPool(ctx sdk.Context, pool gamm.PoolI, exitingShares sdk.Int, exitFee sdk.Dec) (sdk.Coins, error) {
	totalShares := pool.GetTotalShares()
	if exitingShares.GTE(totalShares) {
		return sdk.Coins{}, sdkioerrors.Wrapf(gamm.ErrLimitMaxAmount, errMsgFormatSharesLargerThanMax, exitingShares, totalShares)
	}

	// refundedShares = exitingShares * (1 - exit fee)
	// with 0 exit fee optimization
	var refundedShares sdk.Dec
	if !exitFee.IsZero() {
		// exitingShares * (1 - exit fee)
		oneSubExitFee := sdk.OneDec().SubMut(exitFee)
		refundedShares = oneSubExitFee.MulIntMut(exitingShares)
	} else {
		refundedShares = sdk.NewDecFromInt(exitingShares)
	}

	shareOutRatio := refundedShares.QuoInt(totalShares)
	// exitedCoins = shareOutRatio * pool liquidity
	exitedCoins := sdk.Coins{}
	poolLiquidity := pool.GetTotalPoolLiquidity(ctx)

	for _, asset := range poolLiquidity {
		// round down here, due to not wanting to over-exit
		exitAmt := shareOutRatio.MulInt(asset.Amount).TruncateInt()
		if exitAmt.LTE(sdk.ZeroInt()) {
			continue
		}
		if exitAmt.GTE(asset.Amount) {
			return sdk.Coins{}, errors.New("too many shares out")
		}
		exitedCoins = exitedCoins.Add(sdk.NewCoin(asset.Denom, exitAmt))
	}

	return exitedCoins, nil
}

// MaximalExactRatioJoin calculates the maximal amount of tokens that can be joined whilst maintaining pool asset's ratio
// returning the number of shares that'd be and how many coins would be left over.
//
//	e.g) suppose we have a pool of 10 foo tokens and 10 bar tokens, with the total amount of 100 shares.
//		 if `tokensIn` provided is 1 foo token and 2 bar tokens, `MaximalExactRatioJoin`
//		 would be returning (10 shares, 1 bar token, nil)
//
// This can be used when `tokensIn` are not guaranteed the same ratio as assets in the pool.
// Calculation for this is done in the following steps.
//  1. iterate through all the tokens provided as an argument, calculate how much ratio it accounts for the asset in the pool
//  2. get the minimal share ratio that would work as the benchmark for all tokens.
//  3. calculate the number of shares that could be joined (total share * min share ratio), return the remaining coins
func MaximalExactRatioJoin(p gamm.PoolI, ctx sdk.Context, tokensIn sdk.Coins) (numShares sdk.Int, remCoins sdk.Coins, err error) {
	coinShareRatios := make([]sdk.Dec, len(tokensIn))
	minShareRatio := sdk.MaxSortableDec
	maxShareRatio := sdk.ZeroDec()

	poolLiquidity := p.GetTotalPoolLiquidity(ctx)
	totalShares := p.GetTotalShares()

	for i, coin := range tokensIn {
		// Note: QuoInt implements floor division, unlike Quo
		// This is because it calls the native golang routine big.Int.Quo
		// https://pkg.go.dev/math/big#Int.Quo
		shareRatio := sdk.NewDecFromInt(coin.Amount).QuoInt(poolLiquidity.AmountOfNoDenomValidation(coin.Denom))
		if shareRatio.LT(minShareRatio) {
			minShareRatio = shareRatio
		}
		if shareRatio.GT(maxShareRatio) {
			maxShareRatio = shareRatio
		}
		coinShareRatios[i] = shareRatio
	}

	if minShareRatio.Equal(sdk.MaxSortableDec) {
		return numShares, remCoins, errors.New("unexpected error in MaximalExactRatioJoin")
	}

	remCoins = sdk.Coins{}
	// critically we round down here (TruncateInt), to ensure that the returned LP shares
	// are always less than or equal to % liquidity added.
	numShares = minShareRatio.MulInt(totalShares).TruncateInt()

	// if we have multiple share values, calculate remainingCoins
	if !minShareRatio.Equal(maxShareRatio) {
		// we have to calculate remCoins
		for i, coin := range tokensIn {
			// if coinShareRatios[i] == minShareRatio, no remainder
			if coinShareRatios[i].Equal(minShareRatio) {
				continue
			}

			usedAmount := minShareRatio.MulInt(poolLiquidity.AmountOfNoDenomValidation(coin.Denom)).Ceil().TruncateInt()
			newAmt := coin.Amount.Sub(usedAmount)
			// if newAmt is non-zero, add to RemCoins. (It could be zero due to rounding)
			if !newAmt.IsZero() {
				remCoins = remCoins.Add(sdk.Coin{Denom: coin.Denom, Amount: newAmt})
			}
		}
	}

	return numShares, remCoins, nil
}

// We binary search a number of LP shares, s.t. if we exited the pool with the updated liquidity,
// and swapped all the tokens back to the input denom, we'd get the same amount. (under 0 swap fee)
// Thanks to CFMM path-independence, we can estimate slippage with these swaps to be sure to get the right numbers here.
// (by path-independence, swap all of B -> A, and then swap all of C -> A will yield same amount of A, regardless
// of order and interleaving)
//
// This implementation requires each of pool.GetTotalPoolLiquidity, pool.ExitPool, and pool.SwapExactAmountIn
// to not update or read from state, and instead only do updates based upon the pool struct.
func BinarySearchSingleAssetJoin(
	pool gamm.PoolI,
	tokenIn sdk.Coin,
	poolWithAddedLiquidityAndShares func(newLiquidity sdk.Coin, newShares sdk.Int) gamm.PoolI,
) (numLPShares sdk.Int, err error) {
	// use dummy context
	ctx := sdk.Context{}
	// Need to get something that makes the result correct within 1 LP share
	// If we fail to reach it within maxIterations, we return an error
	correctnessThreshold := sdk.NewInt(2)
	maxIterations := 300
	// upperbound of number of LP shares = existingShares * tokenIn.Amount / pool.totalLiquidity.AmountOf(tokenIn.Denom)
	existingTokenLiquidity := pool.GetTotalPoolLiquidity(ctx).AmountOf(tokenIn.Denom)
	existingLPShares := pool.GetTotalShares()
	LPShareUpperBound := sdk.NewDecFromInt(existingLPShares.Mul(tokenIn.Amount)).QuoInt(existingTokenLiquidity).Ceil().TruncateInt()
	LPShareLowerBound := sdk.ZeroInt()

	// Creates a pool with tokenIn liquidity added, where it created `sharesIn` number of shares.
	// Returns how many tokens you'd get, if you then exited all of `sharesIn` for tokenIn.Denom
	estimateCoinOutGivenShares := func(sharesIn sdk.Int) (tokenOut sdk.Int, err error) {
		// new pool with added liquidity & LP shares, which we can mutate.
		poolWithUpdatedLiquidity := poolWithAddedLiquidityAndShares(tokenIn, sharesIn)
		swapToDenom := tokenIn.Denom
		// so now due to correctness of exitPool, we exitPool and swap all remaining assets to base asset
		exitFee := sdk.ZeroDec()
		exitedCoins, err := poolWithUpdatedLiquidity.ExitPool(ctx, sharesIn, exitFee)
		if err != nil {
			return sdk.Int{}, err
		}

		return swapAllCoinsToSingleAsset(poolWithUpdatedLiquidity, ctx, exitedCoins, swapToDenom)
	}
	// TODO: Come back and revisit err tolerance
	errTolerance := osmoutils.ErrTolerance{AdditiveTolerance: correctnessThreshold, MultiplicativeTolerance: sdk.Dec{}}
	numLPShares, err = osmoutils.BinarySearch(
		estimateCoinOutGivenShares,
		LPShareLowerBound, LPShareUpperBound, tokenIn.Amount, errTolerance, maxIterations)

	return numLPShares, err
}

func swapAllCoinsToSingleAsset(pool gamm.PoolI, ctx sdk.Context, inTokens sdk.Coins, swapToDenom string) (sdk.Int, error) {
	swapFee := sdk.ZeroDec()
	tokenOutAmt := inTokens.AmountOfNoDenomValidation(swapToDenom)
	for _, coin := range inTokens {
		if coin.Denom == swapToDenom {
			continue
		}
		tokenOut, err := pool.SwapOutAmtGivenIn(ctx, sdk.NewCoins(coin), swapToDenom, swapFee)
		if err != nil {
			return sdk.Int{}, err
		}
		tokenOutAmt = tokenOutAmt.Add(tokenOut.Amount)
	}
	return tokenOutAmt, nil
}
