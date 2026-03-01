package vm

// Opcode definitions for the BVM stack machine.
const (
	// Arithmetic
	STOP byte = 0x00
	ADD  byte = 0x01
	MUL  byte = 0x02
	SUB  byte = 0x03
	DIV  byte = 0x04
	SDIV byte = 0x05
	MOD  byte = 0x06
	SMOD byte = 0x07
	ADDMOD byte = 0x08
	MULMOD byte = 0x09
	EXP    byte = 0x0A
	SIGNEXTEND byte = 0x0B

	// Comparison & Bitwise
	LT     byte = 0x10
	GT     byte = 0x11
	SLT    byte = 0x12
	SGT    byte = 0x13
	EQ     byte = 0x14
	ISZERO byte = 0x15
	AND    byte = 0x16
	OR     byte = 0x17
	XOR    byte = 0x18
	NOT    byte = 0x19
	BYTE   byte = 0x1A
	SHL    byte = 0x1B
	SHR    byte = 0x1C
	SAR    byte = 0x1D

	// SHA3
	SHA3 byte = 0x20

	// Environmental
	ADDRESS      byte = 0x30
	BALANCE      byte = 0x31
	ORIGIN       byte = 0x32
	CALLER       byte = 0x33
	CALLVALUE    byte = 0x34
	CALLDATALOAD byte = 0x35
	CALLDATASIZE byte = 0x36
	CALLDATACOPY byte = 0x37
	CODESIZE     byte = 0x38
	CODECOPY     byte = 0x39
	GASPRICE     byte = 0x3A
	RETURNDATASIZE byte = 0x3D
	RETURNDATACOPY byte = 0x3E

	// Block Information
	BLOCKHASH   byte = 0x40
	COINBASE    byte = 0x41
	TIMESTAMP   byte = 0x42
	NUMBER      byte = 0x43
	PREVRANDAO  byte = 0x44
	GASLIMIT    byte = 0x45
	CHAINID     byte = 0x46
	SELFBALANCE byte = 0x47
	BASEFEE     byte = 0x48

	// Stack, Memory, Storage, Flow
	POP      byte = 0x50
	MLOAD    byte = 0x51
	MSTORE   byte = 0x52
	MSTORE8  byte = 0x53
	SLOAD    byte = 0x54
	SSTORE   byte = 0x55
	JUMP     byte = 0x56
	JUMPI    byte = 0x57
	PC       byte = 0x58
	MSIZE    byte = 0x59
	GAS      byte = 0x5A
	JUMPDEST byte = 0x5B

	// Push
	PUSH0  byte = 0x5F
	PUSH1  byte = 0x60
	PUSH2  byte = 0x61
	PUSH3  byte = 0x62
	PUSH4  byte = 0x63
	PUSH5  byte = 0x64
	PUSH6  byte = 0x65
	PUSH7  byte = 0x66
	PUSH8  byte = 0x67
	PUSH9  byte = 0x68
	PUSH10 byte = 0x69
	PUSH11 byte = 0x6A
	PUSH12 byte = 0x6B
	PUSH13 byte = 0x6C
	PUSH14 byte = 0x6D
	PUSH15 byte = 0x6E
	PUSH16 byte = 0x6F
	PUSH17 byte = 0x70
	PUSH18 byte = 0x71
	PUSH19 byte = 0x72
	PUSH20 byte = 0x73
	PUSH21 byte = 0x74
	PUSH22 byte = 0x75
	PUSH23 byte = 0x76
	PUSH24 byte = 0x77
	PUSH25 byte = 0x78
	PUSH26 byte = 0x79
	PUSH27 byte = 0x7A
	PUSH28 byte = 0x7B
	PUSH29 byte = 0x7C
	PUSH30 byte = 0x7D
	PUSH31 byte = 0x7E
	PUSH32 byte = 0x7F

	// Dup
	DUP1  byte = 0x80
	DUP2  byte = 0x81
	DUP3  byte = 0x82
	DUP4  byte = 0x83
	DUP5  byte = 0x84
	DUP6  byte = 0x85
	DUP7  byte = 0x86
	DUP8  byte = 0x87
	DUP9  byte = 0x88
	DUP10 byte = 0x89
	DUP11 byte = 0x8A
	DUP12 byte = 0x8B
	DUP13 byte = 0x8C
	DUP14 byte = 0x8D
	DUP15 byte = 0x8E
	DUP16 byte = 0x8F

	// Swap
	SWAP1  byte = 0x90
	SWAP2  byte = 0x91
	SWAP3  byte = 0x92
	SWAP4  byte = 0x93
	SWAP5  byte = 0x94
	SWAP6  byte = 0x95
	SWAP7  byte = 0x96
	SWAP8  byte = 0x97
	SWAP9  byte = 0x98
	SWAP10 byte = 0x99
	SWAP11 byte = 0x9A
	SWAP12 byte = 0x9B
	SWAP13 byte = 0x9C
	SWAP14 byte = 0x9D
	SWAP15 byte = 0x9E
	SWAP16 byte = 0x9F

	// Logging
	LOG0 byte = 0xA0
	LOG1 byte = 0xA1
	LOG2 byte = 0xA2
	LOG3 byte = 0xA3
	LOG4 byte = 0xA4

	// Knowledge bridge (Zerone-specific)
	KQUERY  byte = 0xE0
	KVERIFY byte = 0xE1
	KCITE   byte = 0xE2

	// Home bridge (Zerone-specific)
	HQUERY   byte = 0xE3 // Query caller's home
	HMEMORY  byte = 0xE4 // Get home memory CID
	HPARTNER byte = 0xE5 // Get home partnership ID

	// System
	CREATE       byte = 0xF0
	CALL         byte = 0xF1
	CALLCODE     byte = 0xF2
	RETURN       byte = 0xF3
	DELEGATECALL byte = 0xF4
	CREATE2      byte = 0xF5
	STATICCALL   byte = 0xFA
	REVERT       byte = 0xFD
	INVALID      byte = 0xFE
	SELFDESTRUCT byte = 0xFF
)

// OpcodeInfo describes an opcode's metadata.
type OpcodeInfo struct {
	Code            byte
	Mnemonic        string
	StackInputs     int
	StackOutputs    int
	GasCost         uint64
	IsStateModifier bool
	MinVersion      uint32
}

// OpcodeTable maps opcode bytes to their metadata.
var OpcodeTable map[byte]OpcodeInfo

// OpcodeByMnemonic returns the opcode byte for a mnemonic name.
func OpcodeByMnemonic(mnemonic string) (byte, bool) {
	for code, info := range OpcodeTable {
		if info.Mnemonic == mnemonic {
			return code, true
		}
	}
	return 0, false
}

func init() {
	OpcodeTable = map[byte]OpcodeInfo{
		STOP:       {STOP, "STOP", 0, 0, GasZero, false, 1},
		ADD:        {ADD, "ADD", 2, 1, GasVeryLow, false, 1},
		MUL:        {MUL, "MUL", 2, 1, GasLow, false, 1},
		SUB:        {SUB, "SUB", 2, 1, GasVeryLow, false, 1},
		DIV:        {DIV, "DIV", 2, 1, GasLow, false, 1},
		SDIV:       {SDIV, "SDIV", 2, 1, GasLow, false, 1},
		MOD:        {MOD, "MOD", 2, 1, GasLow, false, 1},
		SMOD:       {SMOD, "SMOD", 2, 1, GasLow, false, 1},
		ADDMOD:     {ADDMOD, "ADDMOD", 3, 1, GasMid, false, 1},
		MULMOD:     {MULMOD, "MULMOD", 3, 1, GasMid, false, 1},
		EXP:        {EXP, "EXP", 2, 1, GasExpBase, false, 1},
		SIGNEXTEND: {SIGNEXTEND, "SIGNEXTEND", 2, 1, GasLow, false, 1},
		LT:         {LT, "LT", 2, 1, GasVeryLow, false, 1},
		GT:         {GT, "GT", 2, 1, GasVeryLow, false, 1},
		SLT:        {SLT, "SLT", 2, 1, GasVeryLow, false, 1},
		SGT:        {SGT, "SGT", 2, 1, GasVeryLow, false, 1},
		EQ:         {EQ, "EQ", 2, 1, GasVeryLow, false, 1},
		ISZERO:     {ISZERO, "ISZERO", 1, 1, GasVeryLow, false, 1},
		AND:        {AND, "AND", 2, 1, GasVeryLow, false, 1},
		OR:         {OR, "OR", 2, 1, GasVeryLow, false, 1},
		XOR:        {XOR, "XOR", 2, 1, GasVeryLow, false, 1},
		NOT:        {NOT, "NOT", 1, 1, GasVeryLow, false, 1},
		BYTE:       {BYTE, "BYTE", 2, 1, GasVeryLow, false, 1},
		SHL:        {SHL, "SHL", 2, 1, GasVeryLow, false, 1},
		SHR:        {SHR, "SHR", 2, 1, GasVeryLow, false, 1},
		SAR:        {SAR, "SAR", 2, 1, GasVeryLow, false, 1},
		SHA3:       {SHA3, "SHA3", 2, 1, GasSHA3Base, false, 1},
		ADDRESS:    {ADDRESS, "ADDRESS", 0, 1, GasBase, false, 1},
		BALANCE:    {BALANCE, "BALANCE", 1, 1, GasBalance, false, 1},
		ORIGIN:     {ORIGIN, "ORIGIN", 0, 1, GasBase, false, 1},
		CALLER:     {CALLER, "CALLER", 0, 1, GasBase, false, 1},
		CALLVALUE:  {CALLVALUE, "CALLVALUE", 0, 1, GasBase, false, 1},
		CALLDATALOAD: {CALLDATALOAD, "CALLDATALOAD", 1, 1, GasVeryLow, false, 1},
		CALLDATASIZE: {CALLDATASIZE, "CALLDATASIZE", 0, 1, GasBase, false, 1},
		CALLDATACOPY: {CALLDATACOPY, "CALLDATACOPY", 3, 0, GasVeryLow, false, 1},
		CODESIZE:     {CODESIZE, "CODESIZE", 0, 1, GasBase, false, 1},
		CODECOPY:     {CODECOPY, "CODECOPY", 3, 0, GasVeryLow, false, 1},
		GASPRICE:     {GASPRICE, "GASPRICE", 0, 1, GasBase, false, 1},
		RETURNDATASIZE: {RETURNDATASIZE, "RETURNDATASIZE", 0, 1, GasBase, false, 1},
		RETURNDATACOPY: {RETURNDATACOPY, "RETURNDATACOPY", 3, 0, GasVeryLow, false, 1},
		BLOCKHASH:    {BLOCKHASH, "BLOCKHASH", 1, 1, GasHigh, false, 1},
		COINBASE:     {COINBASE, "COINBASE", 0, 1, GasBase, false, 1},
		TIMESTAMP:    {TIMESTAMP, "TIMESTAMP", 0, 1, GasBase, false, 1},
		NUMBER:       {NUMBER, "NUMBER", 0, 1, GasBase, false, 1},
		PREVRANDAO:   {PREVRANDAO, "PREVRANDAO", 0, 1, GasBase, false, 1},
		GASLIMIT:     {GASLIMIT, "GASLIMIT", 0, 1, GasBase, false, 1},
		CHAINID:      {CHAINID, "CHAINID", 0, 1, GasBase, false, 1},
		SELFBALANCE:  {SELFBALANCE, "SELFBALANCE", 0, 1, GasLow, false, 1},
		BASEFEE:      {BASEFEE, "BASEFEE", 0, 1, GasBase, false, 1},
		POP:          {POP, "POP", 1, 0, GasBase, false, 1},
		MLOAD:        {MLOAD, "MLOAD", 1, 1, GasVeryLow, false, 1},
		MSTORE:       {MSTORE, "MSTORE", 2, 0, GasVeryLow, false, 1},
		MSTORE8:      {MSTORE8, "MSTORE8", 2, 0, GasVeryLow, false, 1},
		SLOAD:        {SLOAD, "SLOAD", 1, 1, GasSloadCost, false, 1},
		SSTORE:       {SSTORE, "SSTORE", 2, 0, GasSstoreSet, true, 1},
		JUMP:         {JUMP, "JUMP", 1, 0, GasMid, false, 1},
		JUMPI:        {JUMPI, "JUMPI", 2, 0, GasHigh, false, 1},
		PC:           {PC, "PC", 0, 1, GasBase, false, 1},
		MSIZE:        {MSIZE, "MSIZE", 0, 1, GasBase, false, 1},
		GAS:          {GAS, "GAS", 0, 1, GasBase, false, 1},
		JUMPDEST:     {JUMPDEST, "JUMPDEST", 0, 0, GasJumpDest, false, 1},
		PUSH0:        {PUSH0, "PUSH0", 0, 1, GasBase, false, 1},
		LOG0:         {LOG0, "LOG0", 2, 0, GasLogBase, true, 1},
		LOG1:         {LOG1, "LOG1", 3, 0, GasLogBase + GasLogTopic, true, 1},
		LOG2:         {LOG2, "LOG2", 4, 0, GasLogBase + 2*GasLogTopic, true, 1},
		LOG3:         {LOG3, "LOG3", 5, 0, GasLogBase + 3*GasLogTopic, true, 1},
		LOG4:         {LOG4, "LOG4", 6, 0, GasLogBase + 4*GasLogTopic, true, 1},
		KQUERY:       {KQUERY, "KQUERY", 1, 2, GasKQuery, false, 1},
		KVERIFY:      {KVERIFY, "KVERIFY", 2, 1, GasKVerify, true, 1},
		KCITE:        {KCITE, "KCITE", 1, 1, GasKCite, true, 1},
		HQUERY:       {HQUERY, "HQUERY", 1, 3, GasHQuery, false, 1},
		HMEMORY:      {HMEMORY, "HMEMORY", 1, 1, GasHMemory, false, 1},
		HPARTNER:     {HPARTNER, "HPARTNER", 1, 1, GasHPartner, false, 1},
		CREATE:       {CREATE, "CREATE", 3, 1, GasCreate, true, 1},
		CALL:         {CALL, "CALL", 7, 1, GasCallBase, false, 1},
		CALLCODE:     {CALLCODE, "CALLCODE", 7, 1, GasCallBase, false, 1},
		RETURN:       {RETURN, "RETURN", 2, 0, GasZero, false, 1},
		DELEGATECALL: {DELEGATECALL, "DELEGATECALL", 6, 1, GasCallBase, false, 1},
		CREATE2:      {CREATE2, "CREATE2", 4, 1, GasCreate, true, 1},
		STATICCALL:   {STATICCALL, "STATICCALL", 6, 1, GasCallBase, false, 1},
		REVERT:       {REVERT, "REVERT", 2, 0, GasZero, false, 1},
		INVALID:      {INVALID, "INVALID", 0, 0, GasZero, false, 1},
		SELFDESTRUCT: {SELFDESTRUCT, "SELFDESTRUCT", 1, 0, GasSelfDestruct, true, 1},
	}

	// Register PUSH1-PUSH32
	for i := byte(0); i < 32; i++ {
		op := PUSH1 + i
		if _, exists := OpcodeTable[op]; !exists {
			OpcodeTable[op] = OpcodeInfo{op, "PUSH" + uitoa(uint64(i+1)), 0, 1, GasVeryLow, false, 1}
		}
	}

	// Register DUP1-DUP16
	for i := byte(0); i < 16; i++ {
		op := DUP1 + i
		if _, exists := OpcodeTable[op]; !exists {
			OpcodeTable[op] = OpcodeInfo{op, "DUP" + uitoa(uint64(i+1)), int(i + 1), int(i + 2), GasVeryLow, false, 1}
		}
	}

	// Register SWAP1-SWAP16
	for i := byte(0); i < 16; i++ {
		op := SWAP1 + i
		if _, exists := OpcodeTable[op]; !exists {
			OpcodeTable[op] = OpcodeInfo{op, "SWAP" + uitoa(uint64(i+1)), int(i + 2), int(i + 2), GasVeryLow, false, 1}
		}
	}
}

func uitoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
