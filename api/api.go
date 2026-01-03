package api

import (
	"fmt"
	"net/http"

	"github.com/BlockLucky/dwg-go/api/api_config"
	"github.com/BlockLucky/dwg-go/api/api_handler"
	"github.com/gorilla/mux"
)

func StartAPIService(apiCfg *api_config.ApiConfig) {

	if apiCfg.Port < 1 || apiCfg.Port > 65535 {
		fmt.Printf("api port must be between 1 and 65535")
		return
	}

	api_config.CurrentApiConfig = apiCfg

	go func() {
		muxRouter := mux.NewRouter()
		muxRouter.Use(api_handler.Middleware) // 使用中间件
		muxRouter.HandleFunc("/", api_handler.HomeHandler).Methods("GET")
		muxRouter.HandleFunc("/api/v1", api_handler.ApiHandler).Methods("POST")

		addr := fmt.Sprintf("%s:%d", "0.0.0.0", apiCfg.Port)
		fmt.Printf("API server Run On  [%s]", fmt.Sprintf("http://127.0.0.1:%d", apiCfg.Port))
		if err := http.ListenAndServe(addr, muxRouter); err != nil {
			fmt.Printf("Failed to start HTTP server: %v\n", err)
		}
	}()

}
