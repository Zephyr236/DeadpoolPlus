package utils

import (
	"math/rand"
	"sync"
)

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
	print(banner)
}
