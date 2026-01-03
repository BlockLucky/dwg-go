package api_response

import (
	"dual_wallet/api/api_rpc"
	"encoding/json"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"net/http"
)

func HandleResponse(w http.ResponseWriter, err error, respData interface{}, reqModel *api_rpc.RPCRequest) {
	w.Header().Set("Content-Type", "application/json")

	aID := "1"
	if reqModel != nil {
		aID = reqModel.ID
	}

	resp := &api_rpc.RPCResponse{
		JsonRPC: "2.0",
		ID:      aID,
	}

	if err != nil {
		errMap := map[string]interface{}{
			"error_code": "-1",
			"error_msg":  err.Error(),
		}
		resp.Error = errMap
	} else {
		resp.Result = respData
	}

	// 直接使用 Encoder 编码并写入响应体
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		// 如果编码失败，可以记录日志或进一步处理
		gtbox_log.LogErrorf("Failed to encode JSON response[%v]", http.StatusInternalServerError)
	}
}
