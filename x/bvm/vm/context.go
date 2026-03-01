package vm

import "math/big"

// ExecutionMode determines call semantics.
type ExecutionMode int

const (
	ModeCall         ExecutionMode = iota
	ModeStaticCall
	ModeDelegateCall
	ModeCallCode
	ModeCreate
)

// ExecutionContext provides runtime parameters to the interpreter.
type ExecutionContext struct {
	Caller          []byte
	Origin          []byte
	CurrentContract []byte
	BlockNumber     uint64
	Timestamp       uint64
	GasPrice        *big.Int
	GasLimit        uint64
	Value           *big.Int
	CallData        []byte
	Mode            ExecutionMode
	Bytecode        []byte
	ChainID         *big.Int
	Coinbase        []byte
	PrevRandao      *big.Int
	BaseFee         *big.Int

	ContractBvmVersion uint32
	GasSchedule        *GasSchedule

	CallerDID    string               // DID of the caller (empty if anonymous)
	Capabilities *SessionCapabilities // nil = deny all agent opcodes (secure default)
}

// SessionCapabilities restricts what a session key can do within BVM execution.
type SessionCapabilities struct {
	CanTransfer     bool
	CanStake        bool
	CanSubmitClaims bool
	CanVote         bool
}

// ExecutionResult is returned after bytecode execution completes.
type ExecutionResult struct {
	Success      bool
	ReturnData   []byte
	GasUsed      uint64
	Logs         []Log
	StateChanges []StateChange
	Error        *ExecutionError
}

type Log struct {
	Address []byte
	Topics  [][]byte
	Data    []byte
}

type StateChange struct {
	Type     StateChangeType
	Address  []byte
	Key      string
	OldValue *big.Int
	NewValue *big.Int
}

type StateChangeType int

const (
	StateChangeStorage StateChangeType = iota
	StateChangeBalance
	StateChangeNonce
	StateChangeCode
)

type ExecutionError struct {
	Code    ErrorCode
	Message string
}

func (e *ExecutionError) Error() string { return e.Message }

type ErrorCode int

const (
	ErrCodeOutOfGas ErrorCode = iota
	ErrCodeStackOverflow
	ErrCodeStackUnderflow
	ErrCodeInvalidOpcode
	ErrCodeInvalidJump
	ErrCodeRevert
	ErrCodeStaticStateChange
	ErrCodeInsufficientBalance
	ErrCodeReturnDataOutOfBounds
	ErrCodeCallDepthExceeded
	ErrCodeStorageLimitExceeded
	ErrCodeExecutionFailed
)

// StateDB is the interface for contract storage access.
type StateDB interface {
	GetStorage(contract []byte, key []byte) []byte
	SetStorage(contract []byte, key []byte, value []byte)
	GetBalance(addr []byte) *big.Int
	GetCode(addr []byte) []byte
	GetCodeSize(addr []byte) int
	GetCodeHash(addr []byte) []byte
	Exists(addr []byte) bool
}

// HostFunctions provides hooks for Zerone-specific opcodes.
type HostFunctions interface {
	// KQUERY (0xE0) — query fact from x/knowledge
	KQuery(factId []byte) (exists bool, confidence uint64, domain []byte)
	// KVERIFY (0xE1) — submit verification vote
	KVerify(callerDID string, claimId, voteHash []byte) bool
	// KCITE (0xE2) — record citation
	KCite(callerDID string, factId []byte) bool

	// HQUERY (0xE3) — query caller's home from x/home
	HQuery(callerAddr []byte) (hasHome bool, homeId []byte, status []byte)
	// HMEMORY (0xE4) — get home memory CID
	HMemory(homeId []byte) []byte
	// HPARTNER (0xE5) — get home partnership ID
	HPartner(homeId []byte) []byte
}

const (
	MaxCallDepth               = 1024
	MaxStorageSlotsPerContract = 16384
	MaxBytecodeSize            = 24576
)
