package builder

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/nervosnetwork/ckb-sdk-go/collector"
	"github.com/nervosnetwork/ckb-sdk-go/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/nervosnetwork/ckb-sdk-go/utils"
	"github.com/pkg/errors"
	"math"
	"math/big"
)

var _ UnsignedTxBuilder = (*WithdrawChequesUnsignedTxBuilder)(nil)

var relativeEpochNumber uint64 = 6

type WithdrawChequesUnsignedTxBuilder struct {
	Sender         *types.Script
	Receiver       *types.Script
	FeeRate        uint64
	CkbIterator    collector.CellCollectionIterator
	ChequeIterator collector.CellCollectionIterator
	SystemScripts  *utils.SystemScripts
	UUID           string
	Client         rpc.Client

	tx                    *types.Transaction
	result                *collector.LiveCellCollectResult
	ckbChangeOutputIndex  *collector.ChangeOutputIndex
	sUDTChangeOutputIndex *collector.ChangeOutputIndex
	groups                [][]int
}

func (b *WithdrawChequesUnsignedTxBuilder) NewTransaction() {
	b.tx = &types.Transaction{}
}

func (b *WithdrawChequesUnsignedTxBuilder) BuildVersion() {
	b.tx.Version = 0
}

func (b *WithdrawChequesUnsignedTxBuilder) BuildHeaderDeps() {
	b.tx.HeaderDeps = []types.Hash{}
}

func (b *WithdrawChequesUnsignedTxBuilder) BuildCellDeps() {
	b.tx.CellDeps = []*types.CellDep{
		{
			OutPoint: b.SystemScripts.SecpSingleSigCell.OutPoint,
			DepType:  types.DepTypeDepGroup,
		},
		{
			OutPoint: b.SystemScripts.SUDTCell.OutPoint,
			DepType:  b.SystemScripts.SUDTCell.DepType,
		},
		{
			OutPoint: b.SystemScripts.ChequeCell.OutPoint,
			DepType:  b.SystemScripts.ChequeCell.DepType,
		},
	}
}

func (b *WithdrawChequesUnsignedTxBuilder) BuildOutputsAndOutputsData() error {
	udtType := &types.Script{
		CodeHash: b.SystemScripts.SUDTCell.CellHash,
		HashType: b.SystemScripts.SUDTCell.HashType,
		Args:     common.FromHex(b.UUID),
	}
	// set ckb change output, default capacity is 100 ckb, withdraw cheque cell need consume sender's live cell.
	b.tx.Outputs = append(b.tx.Outputs, &types.CellOutput{
		Capacity: uint64(100 * math.Pow10(8)),
		Lock:     b.Receiver,
	})
	b.tx.OutputsData = append(b.tx.OutputsData, []byte{})
	// set ckb change output index
	b.ckbChangeOutputIndex = &collector.ChangeOutputIndex{Value: 0}

	// set sudt change output
	b.tx.Outputs = append(b.tx.Outputs, &types.CellOutput{
		Capacity: udtCellCapacity,
		Lock:     b.Receiver,
		Type:     udtType,
	})
	b.tx.OutputsData = append(b.tx.OutputsData, sudtDataPlaceHolder)
	// set sudt change output index
	b.sUDTChangeOutputIndex = &collector.ChangeOutputIndex{Value: 1}

	return nil
}

func (b *WithdrawChequesUnsignedTxBuilder) BuildInputsAndWitnesses() error {
	// collect cheque cell first
	err := b.collectOneChequeCell()
	if err != nil {
		return err
	}

	// then collect ckb cells
	err = b.collectCkbCells()
	if err != nil {
		return err
	}
	return nil
}

func (b *WithdrawChequesUnsignedTxBuilder) collectCkbCells() error {
	lastChequeWitnessIndex := len(b.tx.Witnesses)
	for b.CkbIterator.HasNext() {
		liveCell, err := b.CkbIterator.CurrentItem()
		if err != nil {
			return err
		}
		b.result.Capacity += liveCell.Output.Capacity
		b.result.LiveCells = append(b.result.LiveCells, liveCell)
		input := &types.CellInput{
			Since: 0,
			PreviousOutput: &types.OutPoint{
				TxHash: liveCell.OutPoint.TxHash,
				Index:  liveCell.OutPoint.Index,
			},
		}
		b.tx.Inputs = append(b.tx.Inputs, input)
		b.tx.Witnesses = append(b.tx.Witnesses, []byte{})
		if len(b.tx.Witnesses[lastChequeWitnessIndex]) == 0 {
			b.tx.Witnesses[lastChequeWitnessIndex] = transaction.EmptyWitnessArgPlaceholder
		}
		ok, err := b.isCkbEnough()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		err = b.CkbIterator.Next()
		if err != nil {
			return err
		}
	}
	return errors.New("insufficient ckb balance")
}

// collectOneChequeCell collect the first cheque cell encountered
func (b *WithdrawChequesUnsignedTxBuilder) collectOneChequeCell() error {
	b.result = &collector.LiveCellCollectResult{}
	if !b.ChequeIterator.HasNext() {
		return errors.New("no cheque cells to claim")
	}
	liveCell, err := b.ChequeIterator.CurrentItem()
	if err != nil {
		return err
	}
	b.result.Capacity += liveCell.Output.Capacity
	b.result.LiveCells = append(b.result.LiveCells, liveCell)
	// init totalAmount
	if _, ok := b.result.Options["totalAmount"]; !ok {
		b.result.Options = make(map[string]interface{})
		b.result.Options["totalAmount"] = big.NewInt(0)
	}
	// update sudt total Amount
	err = b.updateTotalAmount(err, liveCell)
	if err != nil {
		return err
	}
	input := &types.CellInput{
		Since: utils.SinceFromRelativeEpochNumber(relativeEpochNumber),
		PreviousOutput: &types.OutPoint{
			TxHash: liveCell.OutPoint.TxHash,
			Index:  liveCell.OutPoint.Index,
		},
	}
	b.tx.Inputs = append(b.tx.Inputs, input)
	b.tx.Witnesses = append(b.tx.Witnesses, []byte{})
	return nil
}

func (b *WithdrawChequesUnsignedTxBuilder) updateTotalAmount(err error, liveCell *indexer.LiveCell) error {
	amount, err := utils.ParseSudtAmount(liveCell.OutputData)
	if err != nil {
		return errors.WithMessage(err, "sudt amount parse error")
	}
	totalAmount := b.result.Options["totalAmount"].(*big.Int)
	b.result.Options["totalAmount"] = big.NewInt(0).Add(totalAmount, amount)
	return nil
}

func (b *WithdrawChequesUnsignedTxBuilder) UpdateChangeOutput() error {
	// update sudt claim output
	totalAmount := b.result.Options["totalAmount"].(*big.Int)
	b.tx.OutputsData[b.sUDTChangeOutputIndex.Value] = utils.GenerateSudtAmount(totalAmount)

	// then update ckb change output
	fee, err := transaction.CalculateTransactionFee(b.tx, b.FeeRate)
	if err != nil {
		return err
	}
	changeCapacity := b.result.Capacity - b.tx.OutputsCapacity() - fee
	b.tx.Outputs[b.ckbChangeOutputIndex.Value].Capacity = changeCapacity
	err = b.generateGroups()
	if err != nil {
		return err
	}
	return nil
}

func (b *WithdrawChequesUnsignedTxBuilder) GetResult() (*types.Transaction, [][]int) {
	return b.tx, b.groups
}

func (b *WithdrawChequesUnsignedTxBuilder) isCkbEnough() (bool, error) {
	changeCapacity := b.result.Capacity - b.tx.OutputsCapacity()
	if changeCapacity > 0 {
		fee, err := transaction.CalculateTransactionFee(b.tx, b.FeeRate)
		if err != nil {
			return false, err
		}
		changeCapacity -= fee
		changeOutput := b.tx.Outputs[b.ckbChangeOutputIndex.Value]
		changeOutputData := b.tx.OutputsData[b.ckbChangeOutputIndex.Value]
		changeOutputCapacity := changeOutput.OccupiedCapacity(changeOutputData)
		if changeCapacity >= changeOutputCapacity {
			return true, nil
		} else {
			return false, nil
		}
	} else {
		return false, nil
	}
}

func (b *WithdrawChequesUnsignedTxBuilder) generateGroups() error {
	groupInfo := make(map[string][]int)
	senderLockHash, err := b.Sender.Hash()
	if err != nil {
		return err
	}
	for i, liveCell := range b.result.LiveCells {
		lockHash, err := liveCell.Output.Lock.Hash()
		if err != nil {
			return err
		}
		key := lockHash.String()
		if key != senderLockHash.String() {
			continue
		}
		if v, ok := groupInfo[key]; ok {
			v = append(v, i)
			groupInfo[key] = v
		} else {
			groupInfo[key] = []int{i}
		}
	}
	var groups [][]int
	for _, group := range groupInfo {
		groups = append(groups, group)
	}
	b.groups = groups
	return nil
}
