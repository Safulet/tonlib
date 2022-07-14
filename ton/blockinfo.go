package ton

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/Safulet/tonlib/liteclient/tlb"
	"github.com/Safulet/tonlib/tvm/cell"
)

var ErrBlockNotFound = errors.New("block not found")

func (c *APIClient) GetMasterchainInfo(ctx context.Context) (*tlb.BlockInfo, error) {
	resp, err := c.client.Do(ctx, _GetMasterchainInfo, nil)
	if err != nil {
		return nil, err
	}

	block := new(tlb.BlockInfo)
	_, err = block.Load(resp.Data)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// GetBlockInfo DEPRECATED and will be removed soon, please use GetMasterchainInfo
func (c *APIClient) GetBlockInfo(ctx context.Context) (*tlb.BlockInfo, error) {
	return c.GetMasterchainInfo(ctx)
}

func (c *APIClient) LookupBlock(ctx context.Context, workchain int32, shard uint64, seqno uint32) (*tlb.BlockInfo, error) {
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data, 1)
	binary.LittleEndian.PutUint32(data[4:], uint32(workchain))
	binary.LittleEndian.PutUint64(data[8:], shard)
	binary.LittleEndian.PutUint32(data[16:], seqno)

	resp, err := c.client.Do(ctx, _LookupBlock, data)
	if err != nil {
		return nil, err
	}

	switch resp.TypeID {
	case _BlockHeader:
		b := new(tlb.BlockInfo)
		resp.Data, err = b.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		return b, nil
	case _LSError:
		lsErr := LSError{
			Code: binary.LittleEndian.Uint32(resp.Data),
			Text: string(resp.Data[4:]),
		}

		// 651 = block not found code
		if lsErr.Code == 651 {
			return nil, ErrBlockNotFound
		}

		return nil, lsErr
	}

	return nil, errors.New("unknown response type")
}

func (c *APIClient) GetBlockData(ctx context.Context, block *tlb.BlockInfo) (*tlb.Block, error) {
	resp, err := c.client.Do(ctx, _GetBlock, block.Serialize())
	if err != nil {
		return nil, err
	}

	switch resp.TypeID {
	case _BlockData:
		b := new(tlb.BlockInfo)
		resp.Data, err = b.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		var payload []byte
		payload, resp.Data = loadBytes(resp.Data)

		cl, err := cell.FromBOC(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to parse block boc: %w", err)
		}

		var bData tlb.Block
		if err = tlb.LoadFromCell(&bData, cl.BeginParse()); err != nil {
			return nil, fmt.Errorf("failed to parse block data: %w", err)
		}

		return &bData, nil
	case _LSError:
		return nil, LSError{
			Code: binary.LittleEndian.Uint32(resp.Data),
			Text: string(resp.Data[4:]),
		}
	}

	return nil, errors.New("unknown response type")
}

func (c *APIClient) GetBlockTransactions(ctx context.Context, block *tlb.BlockInfo, count uint32, after ...*tlb.TransactionID) ([]*tlb.TransactionID, bool, error) {
	req := append(block.Serialize(), make([]byte, 8)...)

	mode := uint32(0b111)
	if after != nil && after[0] != nil {
		mode |= 1 << 7
	}

	binary.LittleEndian.PutUint32(req[len(req)-8:], mode)
	binary.LittleEndian.PutUint32(req[len(req)-4:], count)
	if len(after) > 0 && after[0] != nil {
		req = append(req, after[0].AccountID...)

		ltBts := make([]byte, 8)
		binary.LittleEndian.PutUint64(ltBts, after[0].LT)
		req = append(req, ltBts...)
	}

	resp, err := c.client.Do(ctx, _ListBlockTransactions, req)
	if err != nil {
		return nil, false, err
	}

	switch resp.TypeID {
	case _BlockTransactions:
		b := new(tlb.BlockInfo)
		resp.Data, err = b.Load(resp.Data)
		if err != nil {
			return nil, false, err
		}

		_ = binary.LittleEndian.Uint32(resp.Data)
		resp.Data = resp.Data[4:]

		incomplete := int32(binary.LittleEndian.Uint32(resp.Data)) == _BoolTrue
		resp.Data = resp.Data[4:]

		vecLn := binary.LittleEndian.Uint32(resp.Data)
		resp.Data = resp.Data[4:]

		txList := make([]*tlb.TransactionID, vecLn)
		for i := 0; i < int(vecLn); i++ {
			mode := binary.LittleEndian.Uint32(resp.Data)
			resp.Data = resp.Data[4:]

			tid := &tlb.TransactionID{}

			if mode&0b1 != 0 {
				tid.AccountID = resp.Data[:32]
				resp.Data = resp.Data[32:]
			}

			if mode&0b10 != 0 {
				tid.LT = binary.LittleEndian.Uint64(resp.Data)
				resp.Data = resp.Data[8:]
			}

			if mode&0b100 != 0 {
				tid.Hash = resp.Data[:32]
				resp.Data = resp.Data[32:]
			}

			txList[i] = tid
		}

		var proof []byte
		proof, resp.Data = loadBytes(resp.Data)
		_ = proof

		return txList, incomplete, nil
	case _LSError:
		return nil, false, LSError{
			Code: binary.LittleEndian.Uint32(resp.Data),
			Text: string(resp.Data[4:]),
		}
	}

	return nil, false, errors.New("unknown response type")
}

func (c *APIClient) GetBlockShardsInfo(ctx context.Context, block *tlb.BlockInfo) ([]*tlb.BlockInfo, error) {
	resp, err := c.client.Do(ctx, _GetAllShardsInfo, block.Serialize())
	if err != nil {
		return nil, err
	}

	switch resp.TypeID {
	case _AllShardsInfo:
		b := new(tlb.BlockInfo)
		resp.Data, err = b.Load(resp.Data)
		if err != nil {
			return nil, err
		}

		var proof []byte
		proof, resp.Data = loadBytes(resp.Data)
		_ = proof

		var data []byte
		data, resp.Data = loadBytes(resp.Data)

		c, err := cell.FromBOC(data)
		if err != nil {
			return nil, err
		}

		var inf tlb.AllShardsInfo
		err = tlb.LoadFromCell(&inf, c.BeginParse())
		if err != nil {
			return nil, err
		}

		var shards []*tlb.BlockInfo

		for _, kv := range inf.ShardHashes.All() {
			var binTree tlb.BinTree
			err = binTree.LoadFromCell(kv.Value.BeginParse().MustLoadRef())
			if err != nil {
				return nil, fmt.Errorf("load BinTree err: %w", err)
			}

			for _, bk := range binTree.All() {
				var bData tlb.ShardDesc
				if err = tlb.LoadFromCell(&bData, bk.Value.BeginParse()); err != nil {
					return nil, fmt.Errorf("load ShardDesc err: %w", err)
				}

				// TODO: its only 9223372036854775808 shard now, need to parse ids from somewhere
				shards = append(shards, &tlb.BlockInfo{
					Workchain: 0,
					Shard:     9223372036854775808,
					SeqNo:     bData.SeqNo,
					RootHash:  bData.RootHash,
					FileHash:  bData.FileHash,
				})
			}
		}

		return shards, nil
	case _LSError:
		return nil, LSError{
			Code: binary.LittleEndian.Uint32(resp.Data),
			Text: string(resp.Data[4:]),
		}
	}

	return nil, errors.New("unknown response type")
}
