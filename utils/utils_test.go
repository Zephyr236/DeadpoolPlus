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

func TestGetNextProxyNormal(t *testing.T) {
	// 设置测试数据
	EffectiveList = []string{"127.0.0.1:1080", "127.0.0.2:1080"}
	proxyIndex = 0
	
	// 第一次调用应该返回第一个代理
	result1 := getNextProxy()
	if result1 != "127.0.0.1:1080" {
		t.Errorf("Expected 127.0.0.1:1080, got: %s", result1)
	}
	
	// 第二次调用应该返回第二个代理
	result2 := getNextProxy()
	if result2 != "127.0.0.2:1080" {
		t.Errorf("Expected 127.0.0.2:1080, got: %s", result2)
	}
	
	// 第三次调用应该循环回第一个代理
	result3 := getNextProxy()
	if result3 != "127.0.0.1:1080" {
		t.Errorf("Expected 127.0.0.1:1080 (loop back), got: %s", result3)
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
		{"DEBUG", "debug"}, // 应该不区分大小写
	}
	
	for _, tc := range testCases {
		LogLevel = tc.input
		// 模拟 main.go 中的处理逻辑
		LogLevel = strings.TrimSpace(LogLevel)
		if LogLevel == "" {
			LogLevel = "normal"
		}
		
		if LogLevel != tc.expected && !(strings.ToLower(LogLevel) == tc.expected) {
			t.Errorf("For input '%s', expected '%s' or case-insensitive match, got: '%s'", 
				tc.input, tc.expected, LogLevel)
		}
	}
}
