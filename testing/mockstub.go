package testing

import (
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
	"github.com/s7techlab/cckit/convert"
)

var (
	// ErrChaincodeNotExists occurs when attempting to invoke a nonexostent external chaincode
	ErrChaincodeNotExists = errors.New(`chaincode not exists`)
	// ErrUnknownFromArgsType  occurs when attempting to set unknown args in From func
	ErrUnknownFromArgsType = `unknown args type to cckit.MockStub.From func`
)

// MockStub replacement of shim.MockStub with creator mocking facilities
type MockStub struct {
	shim.MockStub
	cc                      shim.Chaincode
	mockCreator             []byte
	ClearCreatorAfterInvoke bool
	_args                   [][]byte
	InvokablesFull          map[string]*MockStub
	creatorTransformer      func(...interface{}) (mspID, cert string)
}

// NewMockStub creates MockStub
func NewMockStub(name string, cc shim.Chaincode) *MockStub {
	s := shim.NewMockStub(name, cc)
	fs := new(MockStub)
	fs.MockStub = *s
	fs.cc = cc
	fs.InvokablesFull = make(map[string]*MockStub)
	return fs
}

// GetArgs mocked args
func (stub *MockStub) GetArgs() [][]byte {
	return stub._args
}

// SetArgs set mocked args
func (stub *MockStub) SetArgs(args [][]byte) {
	stub._args = args
}

// GetStringArgs get mocked args as strings
func (stub *MockStub) GetStringArgs() []string {
	args := stub.GetArgs()
	strargs := make([]string, 0, len(args))
	for _, barg := range args {
		strargs = append(strargs, string(barg))
	}
	return strargs
}

// MockPeerChaincode link to another MockStub
func (stub *MockStub) MockPeerChaincode(invokableChaincodeName string, otherStub *MockStub) {
	stub.InvokablesFull[invokableChaincodeName] = otherStub
}

// InvokeChaincode using another MockStub
func (stub *MockStub) InvokeChaincode(chaincodeName string, args [][]byte, channel string) peer.Response {

	// TODO "args" here should possibly be a serialized pb.ChaincodeInput
	// Internally we use chaincode name as a composite name
	if channel != "" {
		chaincodeName = chaincodeName + "/" + channel
	}

	otherStub, exists := stub.InvokablesFull[chaincodeName]
	if !exists {
		return shim.Error(ErrChaincodeNotExists.Error())
	}

	res := otherStub.MockInvoke(stub.TxID, args)
	return res
}

// GetFunctionAndParameters mocked
func (stub *MockStub) GetFunctionAndParameters() (function string, params []string) {
	allargs := stub.GetStringArgs()
	function = ""
	params = []string{}
	if len(allargs) >= 1 {
		function = allargs[0]
		params = allargs[1:]
	}
	return
}

// RegisterCreatorTransformer  that transforms creator data to MSP_ID and X.509 certificate
func (stub *MockStub) RegisterCreatorTransformer(transformer func(...interface{}) (mspID, cert string)) *MockStub {
	stub.creatorTransformer = transformer
	return stub
}

// MockCreator of tx
func (stub *MockStub) MockCreator(mspID string, cert string) {
	stub.mockCreator, _ = msp.NewSerializedIdentity(mspID, []byte(cert))
}

func (stub *MockStub) generateTxUID() string {
	return "xxx"
}

// Init func of chaincode - sugared version with autogenerated tx uuid
func (stub *MockStub) Init(iargs ...interface{}) peer.Response {
	args, err := convert.ArgsToBytes(iargs...)
	if err != nil {
		return shim.Error(err.Error())
	}

	return stub.MockInit(stub.generateTxUID(), args)
}

// MockInit mocked init function
func (stub *MockStub) MockInit(uuid string, args [][]byte) peer.Response {

	//default method name
	//if len(args) == 0 || string(args[0]) != "Init" {
	//	args = append([][]byte{[]byte("Init")}, args...)
	//}

	stub.SetArgs(args)
	stub.MockTransactionStart(uuid)
	res := stub.cc.Init(stub)
	stub.MockTransactionEnd(uuid)

	if stub.ClearCreatorAfterInvoke {
		stub.mockCreator = nil
	}

	return res
}

// MockInvoke  mocket init function
func (stub *MockStub) MockInvoke(uuid string, args [][]byte) peer.Response {
	// this is a hack here to set MockStub.args, because its not accessible otherwise
	stub.SetArgs(args)

	// now do the invoke with the correct stub
	stub.MockTransactionStart(uuid)
	res := stub.cc.Invoke(stub)
	stub.MockTransactionEnd(uuid)

	if stub.ClearCreatorAfterInvoke {
		stub.mockCreator = nil
	}
	return res
}

// Invoke sugared invoke function with autogenerated tx uuid
func (stub *MockStub) Invoke(funcName string, iargs ...interface{}) peer.Response {
	fargs, err := convert.ArgsToBytes(iargs...)
	if err != nil {
		return shim.Error(err.Error())
	}
	args := append([][]byte{[]byte(funcName)}, fargs...)
	return stub.MockInvoke(stub.generateTxUID(), args)
}

// GetCreator mocked
func (stub *MockStub) GetCreator() ([]byte, error) {
	return stub.mockCreator, nil
}

// From tx creator mock
func (stub *MockStub) From(mspParams ...interface{}) *MockStub {
	var mspID, cert string

	if stub.creatorTransformer != nil {
		mspID, cert = stub.creatorTransformer(mspParams...)
	} else if len(mspParams) == 1 {

		switch mspParams[0].(type) {

		// array with 2 elements  - mspId and ca cert
		case [2]string:
			mspID = mspParams[0].([2]string)[0]
			cert = mspParams[0].([2]string)[1]
			//stub.MockCreator(mspParams[0].([2]string)[0], mspParams[0].([2]string)[1])
		default:
			panic(ErrUnknownFromArgsType)
		}
	} else if len(mspParams) == 2 {
		mspID = mspParams[0].(string)
		cert = mspParams[1].(string)
	}

	stub.MockCreator(mspID, cert)
	return stub
}