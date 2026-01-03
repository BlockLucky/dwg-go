package api_config

type ApiConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	Port              int      `yaml:"port" json:"port"`
	UserAgentAllowed  []string `yaml:"user_agent_allowed" json:"user_agent_allowed"`
	APIMethodsAllowed []string `yaml:"api_methods_allowed" json:"api_methods_allowed"`
}

var (
	CurrentApiConfig *ApiConfig
)

// CheckAllowedMethods 允许方法
func CheckAllowedMethods(method string) bool {
	for _, v := range CurrentApiConfig.APIMethodsAllowed {
		if v == method {
			return true
		}
	}
	return false
}

// CheckAllowedUserAgent 检查UA是否在白名单
func CheckAllowedUserAgent(uaName string) bool {
	for _, v := range CurrentApiConfig.UserAgentAllowed {
		if v == uaName {
			return true
		}
	}
	return false
}
