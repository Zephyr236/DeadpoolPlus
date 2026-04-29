package utils

import "sync"

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

// GetCurrentProxyIndex 获取当前代理索引
func GetCurrentProxyIndex() int {
	mu.Lock()
	defer mu.Unlock()
	return proxyIndex
}

// SetNextProxyIndex 设置下一个代理索引
func SetNextProxyIndex() {
	mu.Lock()
	defer mu.Unlock()
	if len(EffectiveList) > 0 {
		proxyIndex = (proxyIndex + 1) % len(EffectiveList)
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
