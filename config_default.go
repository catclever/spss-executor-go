//go:build !winprod

package main

func getAppConfig() AppConfig {
	return AppConfig{
		ServerUrl: "ws://localhost:9292",
		SpssPath:  "C:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe",
		LlmModel:  "glm-4.7",
		ApiKey:    "",
	}
}
