//go:build winprod

package main

import _ "embed"

//go:embed key
var defaultKey string

func getAppConfig() AppConfig {
	return AppConfig{
		ServerUrl: "wss://web-production-bc394.up.railway.app",
		SpssPath:  "D:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe",
		LlmModel:  "glm-5",
		ApiKey:    defaultKey,
	}
}
