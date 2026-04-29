package utils

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ProxyStats 单个代理的统计信息
type ProxyStats struct {
	UseCount      int           // 总使用次数
	SuccessCount  int           // 成功次数
	FailCount     int           // 连接失败次数
	TotalRespTime time.Duration // 总响应时间（用于计算平均值）
	LastUsed      time.Time     // 上次使用时间
	FailStreak    int           // 连续失败次数（用于健康度判断）
}

var (
	SocksList     []string
	EffectiveList []string
	proxyIndex    int
	Timeout       int
	LastDataFile  = "lastData.txt"
	LogLevel      = "normal" // normal: 只打印重要信息, debug: 打印每个请求的代理
	Wg            sync.WaitGroup
	mu            sync.Mutex
	semaphore     chan struct{}

	// 代理统计信息：key 为 "IP:PORT"
	StatsMap map[string]*ProxyStats

	// 最大连续失败次数，超过则移除代理（默认3）
	MaxFailCount int

	// 优雅关闭
	ShutdownChan chan struct{}
	ActiveConns  int32 // 当前活跃连接数
	activeMu     sync.Mutex
)

// GetCurrentProxyIndex 获取当前随机选择的代理索引
func GetCurrentProxyIndex() int {
	mu.Lock()
	defer mu.Unlock()
	if len(EffectiveList) == 0 {
		return -1
	}
	// 随机选择一个索引
	proxyIndex = rand.Intn(len(EffectiveList))
	return proxyIndex
}

// SetNextProxyIndex 随机选择下一个代理索引
func SetNextProxyIndex() {
	mu.Lock()
	defer mu.Unlock()
	if len(EffectiveList) > 0 {
		// 随机选择一个索引
		proxyIndex = rand.Intn(len(EffectiveList))
	}
}

func Banner() {
	banner := `
   ____                        __                          ___      
  /\ $_$\                     /\ \                        /\_ \     
  \ \ \/\ \     __     __     \_\ \  _____     ___     ___\//\ \    
   \ \ \ \ \  /@__@\ /^__^\   />_< \/\ -__-\  /*__*\  /'__'\\ \ \   
    \ \ \_\ \/\  __//\ \_\.\_/\ \-\ \ \ \_\ \/\ \-\ \/\ \_\ \\-\ \_ 
     \ \____/\ \____\ \__/.\_\ \___,_\ \ ,__/\ \____/\ \____//\____\
      \/___/  \/____/\/__/\/_/\/__,_ /\ \ \/  \/___/  \/___/ \/____/
                                       \ \_\                        
                                        \/_/                        
`
	fmt.Print(ColorCyan + banner + ColorReset)
}
