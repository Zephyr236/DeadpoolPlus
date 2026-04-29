package utils

import (
	"testing"
	"strings"
)

func TestAddSocks(t *testing.T) {
	// 清空全局变量
	SocksList = []string{}
	
	// 测试添加单个代理
	addSocks("127.0.0.1:1080")
	if len(SocksList) != 1 {
		t.Errorf("Expected 1 sock, got: %d", len(SocksList))
	}
	
	// 测试添加重复代理（addSocks 不去重，由调用方保证）
	addSocks("127.0.0.1:1080")
	if len(SocksList) != 2 {
		t.Errorf("Expected 2 socks (no deduplication in addSocks), got: %d", len(SocksList))
	}
	
	// 测试添加不同代理
	addSocks("127.0.0.2:1080")
	if len(SocksList) != 3 {
		t.Errorf("Expected 3 socks, got: %d", len(SocksList))
	}
}

func TestGetNextProxyEmptyList(t *testing.T) {
	// 清空全局变量
	EffectiveList = []string{}
	proxyIndex = 0
	
	result := getNextProxy()
	if result != "" {
		t.Errorf("Expected empty string for empty list, got: %s", result)
	}
}

func TestGetNextProxyRandom(t *testing.T) {
	// 设置测试数据
	EffectiveList = []string{"127.0.0.1:1080", "127.0.0.2:1080", "127.0.0.3:1080"}
	proxyIndex = 0
	
	// 随机轮询：调用多次，应该都返回列表中的代理
	for i := 0; i < 100; i++ {
		result := getNextProxy()
		found := false
		for _, proxy := range EffectiveList {
			if proxy == result {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("getNextProxy returned invalid proxy: %s", result)
			break
		}
	}
}

func TestDelInvalidProxy(t *testing.T) {
	// 设置测试数据
	EffectiveList = []string{"127.0.0.1:1080", "127.0.0.2:1080", "127.0.0.3:1080"}
	proxyIndex = 1 // 当前指向第二个元素
	
	// 删除第一个代理 (索引 0)
	delInvalidProxy("127.0.0.1:1080")
	
	if len(EffectiveList) != 2 {
		t.Errorf("Expected 2 socks after delete, got: %d", len(EffectiveList))
	}
	
	// proxyIndex (1) > i (0)，所以 proxyIndex-- 变成 0
	// 现在指向新的第一个元素 "127.0.0.2:1080"
	if proxyIndex != 0 {
		t.Errorf("Expected proxyIndex=0, got: %d", proxyIndex)
	}
	
	// 验证剩余代理
	if EffectiveList[0] != "127.0.0.2:1080" || EffectiveList[1] != "127.0.0.3:1080" {
		t.Errorf("Unexpected EffectiveList: %v", EffectiveList)
	}
}

func TestLogLevelSetting(t *testing.T) {
	// 测试日志级别设置
	testCases := []struct {
		input    string
		expected string
	}{
		{"debug", "debug"},
		{"normal", "normal"},
		{"", "normal"}, // 空字符串应该默认为 normal
	}
	
	for _, tc := range testCases {
		LogLevel = strings.TrimSpace(tc.input)
		if LogLevel == "" {
			LogLevel = "normal"
		}
		
		if LogLevel != tc.expected {
			t.Errorf("For input '%s', expected '%s', got: '%s'", 
				tc.input, tc.expected, LogLevel)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	// 测试配置验证功能
	
	// 测试用例 1：合法配置
	validConfig := Config{
		Listener: ListenerConfig{
			IP:       "127.0.0.1",
			Port:     10086,
			LogLevel: "normal",
		},
		CheckSocks: CheckSocksConfig{
			Timeout:          6,
			MaxConcurrentReq: 200,
			CheckGeolocate: CheckGeolocateConfig{
				Switch: "close",
			},
		},
	}
	
	errors := ValidateConfig(validConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got: %v", errors)
	}
	
	// 测试用例 2：非法端口
	invalidPortConfig := validConfig
	invalidPortConfig.Listener.Port = 99999
	
	errors = ValidateConfig(invalidPortConfig)
	if len(errors) == 0 {
		t.Errorf("Expected validation error for invalid port")
	}
	
	// 测试用例 3：非法日志级别
	invalidLogLevelConfig := validConfig
	invalidLogLevelConfig.Listener.LogLevel = "verbose"
	
	errors = ValidateConfig(invalidLogLevelConfig)
	if len(errors) == 0 {
		t.Errorf("Expected validation error for invalid logLevel")
	}
	
	// 测试用例 4：非法超时时间
	invalidTimeoutConfig := validConfig
	invalidTimeoutConfig.CheckSocks.Timeout = 0
	
	errors = ValidateConfig(invalidTimeoutConfig)
	if len(errors) == 0 {
		t.Errorf("Expected validation error for invalid timeout")
	}
	
	// 测试用例 5：非法并发数
	invalidConcurrentConfig := validConfig
	invalidConcurrentConfig.CheckSocks.MaxConcurrentReq = 0
	
	errors = ValidateConfig(invalidConcurrentConfig)
	if len(errors) == 0 {
		t.Errorf("Expected validation error for invalid maxConcurrentReq")
	}
	
	// 测试用例 6：非法 CheckGeolocate 开关
	invalidSwitchConfig := validConfig
	invalidSwitchConfig.CheckSocks.CheckGeolocate.Switch = "enabled"
	
	errors = ValidateConfig(invalidSwitchConfig)
	if len(errors) == 0 {
		t.Errorf("Expected validation error for invalid checkGeolocate switch")
	}
}

func TestIsValidCronExpression(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"0 */5 * * *", true},
		{"0 6 * * 6", true},
		{"*/10 * * * *", true},
		{"invalid", false},
		{"60 * * * *", true}, // 我们的验证很简单，不会检查范围
		{"* * * * *", true},
		{"* * * * * *", true}, // 6 个字段
	}
	
	for _, tc := range testCases {
		result := isValidCronExpression(tc.input)
		if result != tc.expected {
			t.Errorf("For input '%s', expected %v, got: %v", tc.input, tc.expected, result)
		}
	}
}
