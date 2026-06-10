package utils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	Listener   ListenerConfig   `toml:"listener"`
	Task       TaskConfig       `toml:"task"`
	CheckSocks CheckSocksConfig `toml:"checkSocks"`
	FOFA       FOFAConfig       `toml:"FOFA"`
	QUAKE      QUAKEConfig      `toml:"QUAKE"`
	HUNTER     HUNTERConfig     `toml:"HUNTER"`
}

type ListenerConfig struct {
	IP       string `toml:"IP"`
	Port     int    `toml:"PORT"`
	UserName string `toml:"userName"`
	Password string `toml:"password"`
	LogLevel string `toml:"logLevel"` // normal: 只打印重要信息, debug: 打印每个请求的代理
}

type TaskConfig struct {
	PeriodicChecking string `toml:"periodicChecking"`
	PeriodicGetSocks string `toml:"periodicGetSocks"`
}

type CheckSocksConfig struct {
	CheckURL         string               `toml:"checkURL"`
	CheckRspKeywords string               `toml:"checkRspKeywords"`
	MaxConcurrentReq int                  `toml:"maxConcurrentReq"`
	Timeout          int                  `toml:"timeout"`
	MaxFailCount     int                  `toml:"maxFailCount"` // 连续失败N次才移除代理，默认3
	CheckGeolocate   CheckGeolocateConfig `toml:"checkGeolocate"`
}

type CheckGeolocateConfig struct {
	Switch          string   `toml:"switch"`
	CheckURL        string   `toml:"checkURL"`
	ExcludeKeywords []string `toml:"excludeKeywords"`
	IncludeKeywords []string `toml:"includeKeywords"`
}

type FOFAConfig struct {
	Switch         string   `toml:"switch"`
	APIURL         string   `toml:"apiUrl"`
	Email          string   `toml:"email"`
	Key            string   `toml:"key"`
	QueryStrings   []string `toml:"queryStrings"`     // SOCKS5 查询语句列表
	PoolQuery      string   `toml:"poolQueryString"`  // 代理池搜索语句
	PoolResultSize int      `toml:"poolResultSize"`   // 代理池搜索数量
	ProxyListURLs  []string `toml:"proxyListUrls"`    // 公开代理列表 URL
	ResultSize     int      `toml:"resultSize"`
}

type QUAKEConfig struct {
	Switch      string `toml:"switch"`
	APIURL      string `toml:"apiUrl"`
	Key         string `toml:"key"`
	QueryString string `toml:"queryString"`
	ResultSize  int    `toml:"resultSize"`
}

type HUNTERConfig struct {
	Switch      string `toml:"switch"`
	APIURL      string `toml:"apiUrl"`
	Key         string `toml:"key"`
	QueryString string `toml:"queryString"`
	ResultSize  int    `toml:"resultSize"`
}

func LoadConfig(path string) (Config, error) {
	var config Config
	// 读取并解析 TOML 文件
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = toml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	// 验证配置
	validationErrors := ValidateConfig(config)
	if len(validationErrors) > 0 {
		fmt.Println("⚠️  配置验证警告：")
		for _, e := range validationErrors {
			fmt.Printf("   - %s\n", e)
		}
		fmt.Println()
	}

	return config, err
}

// ValidateConfig 验证配置是否合法
func ValidateConfig(config Config) []string {
	errors := []string{}

	// 1. 验证监听端口
	if config.Listener.Port < 1 || config.Listener.Port > 65535 {
		errors = append(errors, fmt.Sprintf("listener.PORT 必须在 1-65535 之间，当前值: %d", config.Listener.Port))
	}

	// 2. 验证日志级别
	logLevel := strings.TrimSpace(config.Listener.LogLevel)
	if logLevel != "" && logLevel != "normal" && logLevel != "debug" {
		errors = append(errors, fmt.Sprintf("listener.logLevel 只能是 'normal' 或 'debug'，当前值: %s", logLevel))
	}

	// 3. 验证超时时间
	if config.CheckSocks.Timeout < 1 || config.CheckSocks.Timeout > 60 {
		errors = append(errors, fmt.Sprintf("checkSocks.timeout 建议在 1-60 秒之间，当前值: %d", config.CheckSocks.Timeout))
	}

	// 4. 验证并发数
	if config.CheckSocks.MaxConcurrentReq < 1 {
		errors = append(errors, fmt.Sprintf("checkSocks.maxConcurrentReq 必须 > 0，当前值: %d", config.CheckSocks.MaxConcurrentReq))
	}

	// 5. 验证 CheckGeolocate 开关
	geoSwitch := strings.TrimSpace(config.CheckSocks.CheckGeolocate.Switch)
	if geoSwitch != "" && geoSwitch != "open" && geoSwitch != "close" {
		errors = append(errors, fmt.Sprintf("checkSocks.checkGeolocate.switch 只能是 'open' 或 'close'，当前值: %s", geoSwitch))
	}

	// 6. 验证 cron 表达式格式（简单检查）
	if config.Task.PeriodicChecking != "" {
		if !isValidCronExpression(config.Task.PeriodicChecking) {
			errors = append(errors, fmt.Sprintf("task.periodicChecking cron 表达式可能格式不正确: %s", config.Task.PeriodicChecking))
		}
	}
	if config.Task.PeriodicGetSocks != "" {
		if !isValidCronExpression(config.Task.PeriodicGetSocks) {
			errors = append(errors, fmt.Sprintf("task.periodicGetSocks cron 表达式可能格式不正确: %s", config.Task.PeriodicGetSocks))
		}
	}

	// 7. 验证 API 配置（如果开启）
	validateAPIConfig("FOFA", config.FOFA, &errors)
	validateAPIConfig("QUAKE", config.QUAKE, &errors)
	validateAPIConfig("HUNTER", config.HUNTER, &errors)

	return errors
}

// validateAPIConfig 验证 API 配置
func validateAPIConfig(name string, config interface{}, errors *[]string) {
	switch name {
	case "FOFA":
		c := config.(FOFAConfig)
		if strings.TrimSpace(c.Switch) == "open" {
			if strings.TrimSpace(c.Email) == "" {
				*errors = append(*errors, fmt.Sprintf("%s.email 不能为空（switch=open）", name))
			}
			if strings.TrimSpace(c.Key) == "" {
				*errors = append(*errors, fmt.Sprintf("%s.key 不能为空（switch=open）", name))
			}
			if len(c.QueryStrings) == 0 && c.PoolQuery == "" {
				*errors = append(*errors, fmt.Sprintf("%s.queryStrings 和 poolQueryString 至少需要配置一个（switch=open）", name))
			}
			if c.ResultSize < 1 || c.ResultSize > 10000 {
				*errors = append(*errors, fmt.Sprintf("%s.resultSize 必须在 1-10000 之间，当前值: %d", name, c.ResultSize))
			}
		}
	case "QUAKE":
		c := config.(QUAKEConfig)
		if strings.TrimSpace(c.Switch) == "open" {
			if strings.TrimSpace(c.Key) == "" {
				*errors = append(*errors, fmt.Sprintf("%s.key 不能为空（switch=open）", name))
			}
			if c.ResultSize < 1 || c.ResultSize > 10000 {
				*errors = append(*errors, fmt.Sprintf("%s.resultSize 必须在 1-10000 之间，当前值: %d", name, c.ResultSize))
			}
		}
	case "HUNTER":
		c := config.(HUNTERConfig)
		if strings.TrimSpace(c.Switch) == "open" {
			if strings.TrimSpace(c.Key) == "" {
				*errors = append(*errors, fmt.Sprintf("%s.key 不能为空（switch=open）", name))
			}
			if c.ResultSize < 100 || c.ResultSize%100 != 0 {
				*errors = append(*errors, fmt.Sprintf("%s.resultSize 必须是 100 的倍数，当前值: %d", name, c.ResultSize))
			}
		}
	}
}

// isValidCronExpression 简单验证 cron 表达式格式（5-6 个字段）
func isValidCronExpression(expr string) bool {
	fields := strings.Fields(strings.TrimSpace(expr))
	// cron 表达式可以是 5 个字段（分 时 日 月 周）或 6 个字段（秒 分 时 日 月 周）
	if len(fields) < 5 || len(fields) > 6 {
		return false
	}
	// 简单检查：每个字段不应该包含明显的非法字符
	for _, field := range fields {
		matched, _ := regexp.MatchString(`^[\d,\-*/?]+$`, field)
		if !matched {
			return false
		}
	}
	return true
}
