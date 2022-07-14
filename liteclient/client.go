package liteclient

type LiteResponse struct {
	TypeID int32
	Data   []byte
	err    error
}

type LiteRequest struct {
	TypeID   int32
	QueryID  []byte
	Data     []byte
	RespChan chan *LiteResponse
}
