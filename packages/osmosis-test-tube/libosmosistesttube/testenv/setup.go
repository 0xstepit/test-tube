package testenv

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// helpers

	// tendermint
	"cosmossdk.io/errors"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"

	// cosmos-sdk
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	// wasmd

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	// osmosis
	"github.com/osmosis-labs/osmosis/v21/app"
	concentrateliquiditytypes "github.com/osmosis-labs/osmosis/v21/x/concentrated-liquidity/types"
	gammtypes "github.com/osmosis-labs/osmosis/v21/x/gamm/types"
	ibcratelimittypes "github.com/osmosis-labs/osmosis/v21/x/ibc-rate-limit/types"
	incentivetypes "github.com/osmosis-labs/osmosis/v21/x/incentives/types"
	lockuptypes "github.com/osmosis-labs/osmosis/v21/x/lockup/types"
	minttypes "github.com/osmosis-labs/osmosis/v21/x/mint/types"
	poolincentivetypes "github.com/osmosis-labs/osmosis/v21/x/pool-incentives/types"
	poolmanagertypes "github.com/osmosis-labs/osmosis/v21/x/poolmanager/types"
	protorevtypes "github.com/osmosis-labs/osmosis/v21/x/protorev/types"
	superfluidtypes "github.com/osmosis-labs/osmosis/v21/x/superfluid/types"
	tokenfactorytypes "github.com/osmosis-labs/osmosis/v21/x/tokenfactory/types"
	twaptypes "github.com/osmosis-labs/osmosis/v21/x/twap/types"
)

type TestEnv struct {
	App                *app.OsmosisApp
	Ctx                sdk.Context
	ParamTypesRegistry ParamTypeRegistry
	ValPrivs           []*secp256k1.PrivKey
	NodeHome           string
}

// DebugAppOptions is a stub implementing AppOptions
type DebugAppOptions struct{}

// Get implements AppOptions
func (ao DebugAppOptions) Get(o string) interface{} {
	if o == server.FlagTrace {
		return true
	}
	return nil
}

func NewOsmosisApp(nodeHome string) *app.OsmosisApp {
	db := dbm.NewMemDB()

	return app.NewOsmosisApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		nodeHome,
		5,
		DebugAppOptions{},
		app.EmptyWasmOpts,
		baseapp.SetChainID("osmosis-1"),
	)

}

func InitChain(appInstance *app.OsmosisApp) sdk.Context {
	sdk.DefaultBondDenom = "uosmo"
	genesisState := app.GenesisStateWithValSet(appInstance)

	encCfg := app.MakeEncodingConfig()

	// Set up Wasm genesis state
	wasmGen := wasmtypes.GenesisState{
		Params: wasmtypes.Params{
			// Allow store code without gov
			CodeUploadAccess:             wasmtypes.AllowEverybody,
			InstantiateDefaultPermission: wasmtypes.AccessTypeEverybody,
		},
	}
	genesisState[wasmtypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&wasmGen)

	// set staking genesis state
	stakingGenesisState := stakingtypes.GenesisState{}
	appInstance.AppCodec().UnmarshalJSON(genesisState[stakingtypes.ModuleName], &stakingGenesisState)

	// setup validator signing info
	// get all validators

	// Set up staking genesis state
	// stakingParams := stakingtypes.DefaultParams()
	// stakingParams.UnbondingTime = time.Hour * 24 * 7 * 2 // 2 weeks
	// stakingGen := stakingtypes.GenesisState{
	// 	Params: stakingParams,
	// }
	// genesisState[stakingtypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&stakingGen)

	// Set up incentive genesis state
	// lockableDurations := []time.Duration{
	// 	time.Hour * 24,      // 1 day
	// 	time.Hour * 24 * 7,  // 7 day
	// 	time.Hour * 24 * 14, // 14 days
	// }
	// incentivesParams := incentivetypes.DefaultParams()
	// incentivesParams.DistrEpochIdentifier = "day"
	// incentivesGen := incentivetypes.GenesisState{
	// 	Params:            incentivesParams,
	// 	LockableDurations: lockableDurations,
	// }
	// genesisState[incentivetypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&incentivesGen)

	// Set up pool incentives genesis state
	// poolIncentivesParams := poolincentivetypes.DefaultParams()
	// poolIncentivesParams.MintedDenom = "uosmo"
	// poolIncentivesGen := poolincentivetypes.GenesisState{
	// 	Params:            poolIncentivesParams,
	// 	LockableDurations: lockableDurations,
	// }
	// genesisState[poolincentivetypes.ModuleName] = encCfg.Marshaler.MustMarshalJSON(&poolIncentivesGen)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")

	requireNoErr(err)

	concensusParams := simtestutil.DefaultConsensusParams
	concensusParams.Block = &cmtproto.BlockParams{
		MaxBytes: 22020096,
		MaxGas:   -1,
	}

	// replace sdk.DefaultDenom with "uosmo", a bit of a hack, needs improvement
	stateBytes = []byte(strings.Replace(string(stateBytes), "\"stake\"", "\"uosmo\"", -1))

	appInstance.InitChain(
		abci.RequestInitChain{
			Validators:      []abci.ValidatorUpdate{},
			ConsensusParams: concensusParams,
			AppStateBytes:   stateBytes,
			ChainId:         "osmosis-1",
		},
	)

	ctx := appInstance.NewContext(false, cmtproto.Header{Height: 0, ChainID: "osmosis-1", Time: time.Now().UTC()})

	// for each stakingGenesisState.Validators
	for _, validator := range stakingGenesisState.Validators {
		consAddr, err := validator.GetConsAddr()
		requireNoErr(err)
		signingInfo := slashingtypes.NewValidatorSigningInfo(
			consAddr,
			ctx.BlockHeight(),
			0,
			time.Unix(0, 0),
			false,
			0,
		)
		appInstance.SlashingKeeper.SetValidatorSigningInfo(ctx, consAddr, signingInfo)
	}

	return ctx
}

func (env *TestEnv) BeginNewBlock(executeNextEpoch bool, timeIncreaseSeconds uint64) {
	validators := env.App.StakingKeeper.GetAllValidators(env.Ctx)
	valAddrFancy, err := validators[0].GetConsAddr()
	requireNoErr(err)
	valAddr := valAddrFancy.Bytes()

	env.beginNewBlockWithProposer(executeNextEpoch, valAddr, timeIncreaseSeconds)
}

func (env *TestEnv) InitValidator() []byte {
	valPriv, valAddrFancy := env.setupValidator(stakingtypes.Bonded)
	validator, _ := env.App.StakingKeeper.GetValidator(env.Ctx, valAddrFancy)
	valAddr, _ := validator.GetConsAddr()

	env.ValPrivs = append(env.ValPrivs, valPriv)
	err := banktestutil.FundAccount(env.App.BankKeeper, env.Ctx, valAddrFancy.Bytes(), sdk.NewCoins(sdk.NewInt64Coin("uosmo", 9223372036854775807)))
	if err != nil {
		panic(errors.Wrapf(err, "Failed to fund account"))
	}

	return valAddr.Bytes()
}

func (env *TestEnv) GetValidatorAddresses() []string {
	validators := env.App.StakingKeeper.GetAllValidators(env.Ctx)
	var addresses []string
	for _, validator := range validators {
		addresses = append(addresses, validator.OperatorAddress)
	}

	return addresses
}

// beginNewBlockWithProposer begins a new block with a proposer.
func (env *TestEnv) beginNewBlockWithProposer(executeNextEpoch bool, proposer sdk.ValAddress, timeIncreaseSeconds uint64) {
	validator, found := env.App.StakingKeeper.GetValidator(env.Ctx, proposer)

	if !found {
		panic("validator not found")
	}

	valConsAddr, err := validator.GetConsAddr()
	requireNoErr(err)

	valAddr := valConsAddr.Bytes()

	epochIdentifier := env.App.SuperfluidKeeper.GetEpochIdentifier(env.Ctx)
	epoch := env.App.EpochsKeeper.GetEpochInfo(env.Ctx, epochIdentifier)
	newBlockTime := env.Ctx.BlockTime().Add(time.Duration(timeIncreaseSeconds) * time.Second)
	if executeNextEpoch {
		newBlockTime = env.Ctx.BlockTime().Add(epoch.Duration).Add(time.Second)
	}

	header := cmtproto.Header{ChainID: "osmosis-1", Height: env.Ctx.BlockHeight() + 1, Time: newBlockTime}
	newCtx := env.Ctx.WithBlockTime(newBlockTime).WithBlockHeight(env.Ctx.BlockHeight() + 1)
	env.Ctx = newCtx
	lastCommitInfo := abci.CommitInfo{
		Votes: []abci.VoteInfo{{
			Validator:       abci.Validator{Address: valAddr, Power: 1000},
			SignedLastBlock: true,
		}},
	}
	reqBeginBlock := abci.RequestBeginBlock{Header: header, LastCommitInfo: lastCommitInfo}

	env.App.BeginBlock(reqBeginBlock)
	env.Ctx = env.App.NewContext(false, reqBeginBlock.Header)
}

func (env *TestEnv) setupValidator(bondStatus stakingtypes.BondStatus) (*secp256k1.PrivKey, sdk.ValAddress) {
	valPriv := secp256k1.GenPrivKey()
	valPub := valPriv.PubKey()
	valAddr := sdk.ValAddress(valPub.Address())
	bondDenom := env.App.StakingKeeper.GetParams(env.Ctx).BondDenom
	selfBond := sdk.NewCoins(sdk.Coin{Amount: sdk.NewInt(100), Denom: bondDenom})

	err := banktestutil.FundAccount(env.App.BankKeeper, env.Ctx, sdk.AccAddress(valPub.Address()), selfBond)
	requireNoErr(err)

	stakingMsgServer := stakingkeeper.NewMsgServerImpl(env.App.StakingKeeper)
	stakingCoin := sdk.NewCoin(bondDenom, selfBond[0].Amount)
	ZeroCommission := stakingtypes.NewCommissionRates(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec())
	msg, err := stakingtypes.NewMsgCreateValidator(valAddr, valPub, stakingCoin, stakingtypes.Description{}, ZeroCommission, sdk.OneInt())
	requireNoErr(err)

	res, err := stakingMsgServer.CreateValidator(env.Ctx, msg)
	requireNoErr(err)
	requireNoNil("staking handler", res)

	env.App.BankKeeper.SendCoinsFromModuleToModule(env.Ctx, stakingtypes.NotBondedPoolName, stakingtypes.BondedPoolName, sdk.NewCoins(stakingCoin))

	val, found := env.App.StakingKeeper.GetValidator(env.Ctx, valAddr)
	requierTrue("validator found", found)

	val = val.UpdateStatus(bondStatus)
	env.App.StakingKeeper.SetValidator(env.Ctx, val)

	consAddr, err := val.GetConsAddr()
	requireNoErr(err)
	env.setupDefaultValidatorSigningInfo(consAddr)

	return valPriv, valAddr
}

func (env *TestEnv) SetupDefaultValidator() {
	validators := env.App.StakingKeeper.GetAllValidators(env.Ctx)
	valAddrFancy, err := validators[0].GetConsAddr()
	requireNoErr(err)
	env.setupDefaultValidatorSigningInfo(valAddrFancy)
}

func (env *TestEnv) setupDefaultValidatorSigningInfo(consAddr sdk.ConsAddress) {
	signingInfo := slashingtypes.NewValidatorSigningInfo(
		consAddr,
		env.Ctx.BlockHeight(),
		0,
		time.Unix(0, 0),
		false,
		0,
	)
	env.App.SlashingKeeper.SetValidatorSigningInfo(env.Ctx, consAddr, signingInfo)
}

func (env *TestEnv) SetupParamTypes() {
	pReg := env.ParamTypesRegistry

	pReg.RegisterParamSet(&lockuptypes.Params{})
	pReg.RegisterParamSet(&incentivetypes.Params{})
	pReg.RegisterParamSet(&minttypes.Params{})
	pReg.RegisterParamSet(&twaptypes.Params{})
	pReg.RegisterParamSet(&gammtypes.Params{})
	pReg.RegisterParamSet(&ibcratelimittypes.Params{})
	pReg.RegisterParamSet(&tokenfactorytypes.Params{})
	pReg.RegisterParamSet(&superfluidtypes.Params{})
	pReg.RegisterParamSet(&poolincentivetypes.Params{})
	pReg.RegisterParamSet(&protorevtypes.Params{})
	pReg.RegisterParamSet(&poolmanagertypes.Params{})
	pReg.RegisterParamSet(&concentrateliquiditytypes.Params{})
}

func requireNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

func requireNoNil(name string, nilable any) {
	if nilable == nil {
		panic(fmt.Sprintf("%s must not be nil", name))
	}
}

func requierTrue(name string, b bool) {
	if !b {
		panic(fmt.Sprintf("%s must be true", name))
	}
}
