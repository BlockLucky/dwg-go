package api_rpc

// RPCRequest JSON-RPC 请求和响应结构
type RPCRequest struct {
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	JsonRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
}

type RPCResponse struct {
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error"`
	JsonRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
}
