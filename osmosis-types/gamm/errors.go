package gamm

import (
	sdkioerrors "cosmossdk.io/errors"
	"fmt"
)

type PoolDoesNotExistError struct {
	PoolId uint64
}

func (e PoolDoesNotExistError) Error() string {
	return fmt.Sprintf("pool with ID %d does not exist", e.PoolId)
}

// x/gamm module sentinel errors.
var (
	ErrPoolNotFound        = sdkioerrors.Register(ModuleName, 1, "pool not found")
	ErrPoolAlreadyExist    = sdkioerrors.Register(ModuleName, 2, "pool already exist")
	ErrPoolLocked          = sdkioerrors.Register(ModuleName, 3, "pool is locked")
	ErrTooFewPoolAssets    = sdkioerrors.Register(ModuleName, 4, "pool should have at least 2 assets, as they must be swapping between at least two assets")
	ErrTooManyPoolAssets   = sdkioerrors.Register(ModuleName, 5, "pool has too many assets (currently capped at 8 assets per balancer pool and 2 per stableswap)")
	ErrLimitMaxAmount      = sdkioerrors.Register(ModuleName, 6, "calculated amount is larger than max amount")
	ErrLimitMinAmount      = sdkioerrors.Register(ModuleName, 7, "calculated amount is lesser than min amount")
	ErrInvalidMathApprox   = sdkioerrors.Register(ModuleName, 8, "invalid calculated result")
	ErrAlreadyInvalidPool  = sdkioerrors.Register(ModuleName, 9, "destruction on already invalid pool")
	ErrInvalidPool         = sdkioerrors.Register(ModuleName, 10, "attempting to create an invalid pool")
	ErrDenomNotFoundInPool = sdkioerrors.Register(ModuleName, 11, "denom does not exist in pool")
	ErrDenomAlreadyInPool  = sdkioerrors.Register(ModuleName, 12, "denom already exists in the pool")

	ErrEmptyRoutes              = sdkioerrors.Register(ModuleName, 21, "routes not defined")
	ErrEmptyPoolAssets          = sdkioerrors.Register(ModuleName, 22, "PoolAssets not defined")
	ErrNegativeSwapFee          = sdkioerrors.Register(ModuleName, 23, "swap fee is negative")
	ErrNegativeExitFee          = sdkioerrors.Register(ModuleName, 24, "exit fee is negative")
	ErrTooMuchSwapFee           = sdkioerrors.Register(ModuleName, 25, "swap fee should be lesser than 1 (100%)")
	ErrTooMuchExitFee           = sdkioerrors.Register(ModuleName, 26, "exit fee should be lesser than 1 (100%)")
	ErrNotPositiveWeight        = sdkioerrors.Register(ModuleName, 27, "token weight should be greater than 0")
	ErrWeightTooLarge           = sdkioerrors.Register(ModuleName, 28, "user specified token weight should be less than 2^20")
	ErrNotPositiveCriteria      = sdkioerrors.Register(ModuleName, 29, "min out amount or max in amount should be positive")
	ErrNotPositiveRequireAmount = sdkioerrors.Register(ModuleName, 30, "required amount should be positive")
	ErrTooManyTokensOut         = sdkioerrors.Register(ModuleName, 31, "tx is trying to get more tokens out of the pool than exist")
	ErrSpotPriceOverflow        = sdkioerrors.Register(ModuleName, 32, "invalid spot price (overflowed)")
	ErrSpotPriceInternal        = sdkioerrors.Register(ModuleName, 33, "internal spot price error")

	ErrPoolParamsInvalidDenom     = sdkioerrors.Register(ModuleName, 50, "pool params' LBP params has an invalid denomination")
	ErrPoolParamsInvalidNumDenoms = sdkioerrors.Register(ModuleName, 51, "pool params' LBP doesn't have same number of params as underlying pool")

	ErrNotImplemented = sdkioerrors.Register(ModuleName, 60, "function not implemented")

	ErrNotStableSwapPool               = sdkioerrors.Register(ModuleName, 61, "not stableswap pool")
	ErrInvalidStableswapScalingFactors = sdkioerrors.Register(ModuleName, 62, "length between liquidity and scaling factors mismatch")
	ErrNotScalingFactorGovernor        = sdkioerrors.Register(ModuleName, 63, "not scaling factor governor")
	ErrInvalidScalingFactors           = sdkioerrors.Register(ModuleName, 64, "invalid scaling factor")
)
