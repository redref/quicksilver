package types

import (
	fmt "fmt"

	"github.com/ingenuity-build/quicksilver/internal/multierror"
)

func NewGenesisState(params Params) *GenesisState {
	return &GenesisState{Params: params}
}

// DefaultGenesis returns the default ics genesis state
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// ValidateGenesis validates the provided genesis state to ensure the
// expected invariants holds.
func (gs GenesisState) Validate() error {
	errors := make(map[string]error)

	if err := gs.Params.Validate(); err != nil {
		errors["Params"] = err
	}

	for i, claim := range gs.Claims {
		if err := claim.ValidateBasic(); err != nil {
			el := fmt.Sprintf("Claim[%d]", i)
			errors[el] = err
		}
	}

	if len(errors) > 0 {
		return multierror.New(errors)
	}

	return nil
}
