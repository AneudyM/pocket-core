package nodes

import (
	"fmt"
	"github.com/pokt-network/pocket-core/x/nodes/keeper"
	"github.com/pokt-network/posmint/types/module"
	"github.com/pokt-network/posmint/x/supply"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/rpc/client"
	"math/rand"
	fp "path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/pokt-network/pocket-core/x/nodes/types"
	"github.com/pokt-network/posmint/codec"
	"github.com/pokt-network/posmint/store"
	"github.com/pokt-network/posmint/x/auth"
	"github.com/pokt-network/posmint/x/bank"
	"github.com/pokt-network/posmint/x/params"
	//"github.com/pokt-network/posmint/x/supply/internal/types"

	sdk "github.com/pokt-network/posmint/types"
)

// nolint: deadcode unused
var (
	multiPerm    = "multiple permissions account"
	randomPerm   = "random permission"
	holder       = "holder"
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		params.AppModuleBasic{},
		supply.AppModuleBasic{},
	)
	keybaseName = "pocket-keybase"
	kbDirName   = "keybase"
	fs          = string(fp.Separator)
	datadir     string
)

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.Codec {
	var cdc = codec.New()

	bank.RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	supply.RegisterCodec(cdc)
	params.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)

	return cdc
}

func GetTestTendermintClient() client.Client {
	var tmNodeURI string
	var defaultTMURI = "tcp://localhost:26657"

	if tmNodeURI == "" {
		return client.NewHTTP(defaultTMURI, "/websocket")
	}
	return client.NewHTTP(tmNodeURI, "/websocket")
}

// nolint: deadcode unused
func createTestInput(t *testing.T, isCheckTx bool) (sdk.Context, []auth.Account, keeper.Keeper) {
	initPower := int64(100000000000)
	nAccs := int64(4)

	keyAcc := sdk.NewKVStoreKey(auth.StoreKey)
	keyParams := sdk.NewKVStoreKey(params.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(params.TStoreKey)
	keySupply := sdk.NewKVStoreKey(supply.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySupply, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	require.Nil(t, err)

	ctx := sdk.NewContext(ms, abci.Header{ChainID: "test-chain"}, isCheckTx, log.NewNopLogger())
	ctx = ctx.WithConsensusParams(
		&abci.ConsensusParams{
			Validator: &abci.ValidatorParams{
				PubKeyTypes: []string{tmtypes.ABCIPubKeyTypeEd25519},
			},
		},
	)
	cdc := makeTestCodec()

	maccPerms := map[string][]string{
		auth.FeeCollectorName: nil,
		types.StakedPoolName:  {supply.Burner, supply.Staking, supply.Minter},
		types.DAOPoolName:     {supply.Burner, supply.Staking, supply.Minter},
	}
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[supply.NewModuleAddress(acc).String()] = true
	}
	valTokens := sdk.TokensFromConsensusPower(initPower)

	pk := params.NewKeeper(cdc, keyParams, tkeyParams, params.DefaultCodespace)
	ak := auth.NewAccountKeeper(cdc, keyAcc, pk.Subspace(auth.DefaultParamspace), auth.ProtoBaseAccount)
	bk := bank.NewBaseKeeper(ak, pk.Subspace(bank.DefaultParamspace), bank.DefaultCodespace, modAccAddrs)
	sk := supply.NewKeeper(cdc, keySupply, ak, bk, maccPerms)

	moduleManager := module.NewManager(
		auth.NewAppModule(ak),
		bank.NewAppModule(bk, ak),
		supply.NewAppModule(sk, ak),
	)

	genesisState := ModuleBasics.DefaultGenesis()
	moduleManager.InitGenesis(ctx, genesisState)

	posSubSpace := pk.Subspace(keeper.DefaultParamspace)
	initialCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, valTokens))
	accs := createTestAccs(ctx, int(nAccs), initialCoins, &ak)

	keeper := keeper.NewKeeper(cdc, keySupply, bk, sk, posSubSpace, sdk.CodespaceType("pos"))

	params := types.DefaultParams()
	keeper.SetParams(ctx, params)
	return ctx, accs, keeper
}

// nolint: unparam deadcode unused
func createTestAccs(ctx sdk.Context, numAccs int, initialCoins sdk.Coins, ak *auth.AccountKeeper) (accs []auth.Account) {
	for i := 0; i < numAccs; i++ {

		privKey := ed25519.GenPrivKey()
		pubKey := privKey.PubKey()
		addr := sdk.AccAddress(pubKey.Address())
		acc := auth.NewBaseAccountWithAddress(addr)
		acc.Coins = initialCoins
		acc.PubKey = pubKey
		acc.AccountNumber = uint64(i)
		ak.SetAccount(ctx, &acc)
	}
	return
}

//func addMintedCoinsToModule(t *testing.T, ctx sdk.Context, k *keeper.Keeper, module string) {
//	coins := sdk.NewCoins(sdk.NewCoin(k.StakeDenom(ctx), sdk.NewInt(100000000000)))
//	mintErr := k.supplyKeeper.MintCoins(ctx, module, coins.Add(coins))
//	if mintErr != nil {
//		t.Fail()
//	}
//}
//
//func sendFromModuleToAccount(t *testing.T, ctx sdk.Context, k *keeper.Keeper, module string, address sdk.ValAddress, amount sdk.Int) {
//	coins := sdk.NewCoins(sdk.NewCoin(k.StakeDenom(ctx), amount))
//	err := k.supplyKeeper.SendCoinsFromModuleToAccount(ctx, module, sdk.AccAddress(address), coins)
//	if err != nil {
//		t.Fail()
//	}
//}

func getRandomPubKey() ed25519.PubKeyEd25519 {
	var pub ed25519.PubKeyEd25519
	rand.Read(pub[:])
	return pub
}

func getRandomValidatorAddress() sdk.ValAddress {
	return sdk.ValAddress(getRandomPubKey().Address())
}

func getValidator() types.Validator {
	pub := getRandomPubKey()
	return types.Validator{
		Address:      sdk.ValAddress(pub.Address()),
		StakedTokens: sdk.NewInt(100000000000),
		ConsPubKey:   pub,
		Jailed:       false,
		Status:       sdk.Bonded,
		ServiceURL:   "google.com",
		Chains:       []string{"b60d7bdd334cd3768d43f14a05c7fe7e886ba5bcb77e1064530052fed1a3f145"},
	}
}

func getBondedValidator() types.Validator {
	return getValidator()
}

func getUnbondedValidator() types.Validator {
	v := getValidator()
	return v.UpdateStatus(sdk.Unbonded)
}

func getUnbondingValidator() types.Validator {
	v := getValidator()
	return v.UpdateStatus(sdk.Unbonding)
}

func getGenesisStateForTest(ctx sdk.Context, keeper keeper.Keeper, defaultparams bool) types.GenesisState {
	keeper.SetPreviousProposer(ctx, sdk.GetConsAddress(getRandomPubKey()))
	var prm = types.DefaultParams()

	if !defaultparams {
		prm = keeper.GetParams(ctx)
	}
	prevStateTotalPower := keeper.PrevStateValidatorsPower(ctx)
	validators := keeper.GetAllValidators(ctx)
	var prevStateValidatorPowers []types.PrevStatePowerMapping
	keeper.IterateAndExecuteOverPrevStateValsByPower(ctx, func(addr sdk.ValAddress, power int64) (stop bool) {
		prevStateValidatorPowers = append(prevStateValidatorPowers, types.PrevStatePowerMapping{Address: addr, Power: power})
		return false
	})
	signingInfos := make(map[string]types.ValidatorSigningInfo)
	missedBlocks := make(map[string][]types.MissedBlock)
	keeper.IterateAndExecuteOverValSigningInfo(ctx, func(address sdk.ConsAddress, info types.ValidatorSigningInfo) (stop bool) {
		addrstring := address.String()
		signingInfos[addrstring] = info
		localMissedBlocks := []types.MissedBlock{}

		keeper.IterateAndExecuteOverMissedArray(ctx, address, func(index int64, missed bool) (stop bool) {
			localMissedBlocks = append(localMissedBlocks, types.MissedBlock{index, missed})
			return false
		})
		missedBlocks[addrstring] = localMissedBlocks

		return false
	})
	daoTokens := keeper.GetDAOTokens(ctx)
	daoPool := types.DAOPool{Tokens: daoTokens}
	prevProposer := keeper.GetPreviousProposer(ctx)

	return types.GenesisState{
		Params:                   prm,
		PrevStateTotalPower:      prevStateTotalPower,
		PrevStateValidatorPowers: prevStateValidatorPowers,
		Validators:               validators,
		Exported:                 true,
		DAO:                      daoPool,
		SigningInfos:             signingInfos,
		MissedBlocks:             missedBlocks,
		PreviousProposer:         prevProposer,
	}

}

func getSupplyKeeperForTest() supply.Keeper {

	keyAcc := sdk.NewKVStoreKey(auth.StoreKey)
	keyParams := sdk.NewKVStoreKey(params.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(params.TStoreKey)
	keySupply := sdk.NewKVStoreKey(supply.StoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySupply, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	if err != nil {
		fmt.Print("lol")
	}

	ctx := sdk.NewContext(ms, abci.Header{ChainID: "test-chain"}, true, log.NewNopLogger())
	ctx = ctx.WithConsensusParams(
		&abci.ConsensusParams{
			Validator: &abci.ValidatorParams{
				PubKeyTypes: []string{tmtypes.ABCIPubKeyTypeEd25519},
			},
		},
	)
	cdc := makeTestCodec()

	maccPerms := map[string][]string{
		auth.FeeCollectorName: nil,
		types.StakedPoolName:  {supply.Burner, supply.Staking, supply.Minter},
		types.DAOPoolName:     {supply.Burner, supply.Staking, supply.Minter},
	}
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[supply.NewModuleAddress(acc).String()] = true
	}

	pk := params.NewKeeper(cdc, keyParams, tkeyParams, params.DefaultCodespace)
	ak := auth.NewAccountKeeper(cdc, keyAcc, pk.Subspace(auth.DefaultParamspace), auth.ProtoBaseAccount)
	bk := bank.NewBaseKeeper(ak, pk.Subspace(bank.DefaultParamspace), bank.DefaultCodespace, modAccAddrs)
	sk := supply.NewKeeper(cdc, keySupply, ak, bk, maccPerms)

	return sk
}
