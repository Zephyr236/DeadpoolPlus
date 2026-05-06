package utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// 颜色常量 (ANSI escape codes)
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBlue   = "\033[34m"
	ColorWhite  = "\033[37m"
	ColorGray   = "\033[90m"
)

// InitProxyStats 初始化/更新代理统计信息（新增代理时调用，线程安全）
func InitProxyStats(proxies []string) {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap == nil {
		StatsMap = make(map[string]*ProxyStats)
	}
	now := time.Now()
	for _, p := range proxies {
		if _, ok := StatsMap[p]; !ok {
			StatsMap[p] = &ProxyStats{LastUsed: now}
		}
	}
}

// RecordProxySuccess 记录代理成功使用
func RecordProxySuccess(proxyAddr string, respTime time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap == nil {
		return
	}
	s, ok := StatsMap[proxyAddr]
	if !ok {
		s = &ProxyStats{}
		StatsMap[proxyAddr] = s
	}
	s.UseCount++
	s.SuccessCount++
	s.TotalRespTime += respTime
	s.LastUsed = time.Now()
	s.FailStreak = 0 // 成功一次，重置连续失败计数
}

// RecordProxyFailure 记录代理失败，返回 true 表示连续失败次数已达上限应移除
func RecordProxyFailure(proxyAddr string) bool {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap == nil {
		return true // 没有统计信息时，直接移除
	}
	s, ok := StatsMap[proxyAddr]
	if !ok {
		s = &ProxyStats{}
		StatsMap[proxyAddr] = s
	}
	s.UseCount++
	s.FailCount++
	s.FailStreak++
	maxFail := MaxFailCount
	if maxFail <= 0 {
		maxFail = 3 // 默认连续失败3次才移除
	}
	return s.FailStreak >= maxFail
}

// RemoveProxyStats 移除代理时同步清理统计信息
func RemoveProxyStats(proxyAddr string) {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap != nil {
		delete(StatsMap, proxyAddr)
	}
}

// GetSortedProxyStats 返回按使用次数排序的代理统计信息（用于展示）
func GetSortedProxyStats() []ProxyStatsItem {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap == nil {
		return nil
	}
	items := make([]ProxyStatsItem, 0, len(StatsMap))
	for addr, s := range StatsMap {
		avgMs := int64(0)
		if s.SuccessCount > 0 {
			avgMs = int64(s.TotalRespTime / time.Millisecond / time.Duration(s.SuccessCount))
		}
		successRate := 0.0
		if s.UseCount > 0 {
			successRate = float64(s.SuccessCount) / float64(s.UseCount) * 100
		}
		items = append(items, ProxyStatsItem{
			Addr:         addr,
			UseCount:     s.UseCount,
			SuccessCount: s.SuccessCount,
			FailCount:    s.FailCount,
			SuccessRate:  successRate,
			AvgRespTime:  avgMs,
			FailStreak:   s.FailStreak,
			LastUsed:     s.LastUsed,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UseCount > items[j].UseCount // 按使用次数降序
	})
	return items
}

// ProxyStatsItem 用于展示的代理统计项
type ProxyStatsItem struct {
	Addr         string
	UseCount     int
	SuccessCount int
	FailCount    int
	SuccessRate  float64
	AvgRespTime  int64
	FailStreak   int
	LastUsed     time.Time
}

// PrintStats 打印所有代理的统计信息（英文表头+颜色）
func PrintStats() {
	items := GetSortedProxyStats()
	if len(items) == 0 {
		fmt.Println(ColorCyan + "\n[Stats] No proxy statistics available" + ColorReset)
		return
	}
	fmt.Println("\n" + ColorBlue + strings.Repeat("=", 90) + ColorReset)
	fmt.Printf(ColorCyan+"  Proxy Stats (total: %d)\n"+ColorReset, len(items))
	fmt.Println(ColorBlue + strings.Repeat("=", 90) + ColorReset)
	// 英文表头，对齐不会出问题
	fmt.Printf("  %-25s %6s %6s %6s %7s %9s %6s\n",
		"ADDR", "USES", "OK", "FAIL", "RATE", "AVG(ms)", "STREAK")
	fmt.Println(ColorGray + strings.Repeat("-", 80) + ColorReset)
	for _, item := range items {
		if item.UseCount == 0 {
			continue
		}
		// 成功率颜色
		rateColor := ColorGreen
		if item.SuccessRate < 90 {
			rateColor = ColorYellow
		}
		if item.SuccessRate < 50 {
			rateColor = ColorRed
		}
		// 响应时间颜色
		timeColor := ColorGreen
		if item.AvgRespTime > 500 {
			timeColor = ColorYellow
		}
		if item.AvgRespTime > 1000 {
			timeColor = ColorRed
		}
		// 连败颜色
		streakColor := ColorGreen
		if item.FailStreak > 0 {
			streakColor = ColorYellow
		}
		if item.FailStreak >= GetMaxFailCount() {
			streakColor = ColorRed
		}
		fmt.Printf("  %-25s %6d %6d %6d "+rateColor+"%6.1f%%"+ColorReset+" "+timeColor+"%7dms"+ColorReset+" "+streakColor+"%5d"+ColorReset+"\n",
			item.Addr, item.UseCount, item.SuccessCount, item.FailCount,
			item.SuccessRate, item.AvgRespTime, item.FailStreak)
	}
	fmt.Println(ColorBlue + strings.Repeat("=", 90) + ColorReset)
}

// GetMaxFailCount 导出最大失败次数（供PrintStats使用）
func GetMaxFailCount() int {
	return getMaxFailCount()
}

// GetActiveConns 获取当前活跃连接数（用于优雅关闭）
func GetActiveConns() int32 {
	return ActiveConns
}

// 防止goroutine 异步处理问题
var addSocksMu sync.Mutex

func addSocks(socks5 string) {
	addSocksMu.Lock()
	SocksList = append(SocksList, socks5)
	addSocksMu.Unlock()
}

func fetchContent(baseURL string, method string, timeout int, urlParams map[string]string, headers map[string]string, jsonBody string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: time.Duration(timeout) * time.Second,
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if urlParams != nil {
		q := u.Query()
		for key, value := range urlParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}

	var req *http.Request
	if jsonBody != "" {
		req, err = http.NewRequest(method, u.String(), bytes.NewBufferString(jsonBody))
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}

	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36 Edg/112.0.1722.17")
	if len(headers) != 0 {
		for key, value := range headers {
			req.Header.Add(key, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func RemoveDuplicates(list *[]string) {
	seen := make(map[string]struct{})
	var result []string
	for _, sock := range *list {
		if _, ok := seen[sock]; !ok {
			result = append(result, sock)
			seen[sock] = struct{}{}
		}
	}

	*list = result
}

func CheckSocks(checkSocks CheckSocksConfig, socksListParam []string) {
	startTime := time.Now()
	maxConcurrentReq := checkSocks.MaxConcurrentReq
	timeout := checkSocks.Timeout
	semaphore = make(chan struct{}, maxConcurrentReq)

	checkRspKeywords := checkSocks.CheckRspKeywords
	checkGeolocateConfig := checkSocks.CheckGeolocate
	checkGeolocateSwitch := checkGeolocateConfig.Switch
	isOpenGeolocateSwitch := false
	reqUrl := checkSocks.CheckURL
	if checkGeolocateSwitch == "open" {
		isOpenGeolocateSwitch = true
		reqUrl = checkGeolocateConfig.CheckURL
	}
	fmt.Printf(ColorCyan+"时间:[ %v ] 并发:[ %v ],超时标准:[ %vs ]\n"+ColorReset, time.Now().Format("2006-01-02 15:04:05"), maxConcurrentReq, timeout)
	var num int
	total := len(socksListParam)
	var tmpEffectiveList []string
	var tmpMu sync.Mutex
	for _, proxyAddr := range socksListParam {

		Wg.Add(1)
		semaphore <- struct{}{}
		go func(proxyAddr string) {
			tmpMu.Lock()
			num++
			fmt.Printf(ColorCyan+"\r正检测第 [ %v/%v ] 个代理,异步处理中...                    "+ColorReset, num, total)
			tmpMu.Unlock()
			defer Wg.Done()
			defer func() {

				<-semaphore

			}()
			socksProxy := "socks5://" + proxyAddr
			proxy := func(_ *http.Request) (*url.URL, error) {
				return url.Parse(socksProxy)
			}
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				Proxy:           proxy,
			}
			client := &http.Client{
				Transport: tr,
				Timeout:   time.Duration(timeout) * time.Second,
			}
			req, err := http.NewRequest("GET", reqUrl, nil)
			if err != nil {
				return
			}
			req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36 Edg/112.0.1722.17")
			req.Header.Add("referer", "https://www.baidu.com/s?ie=utf-8&f=8&rsv_bp=1&rsv_idx=1&tn=baidu&wd=ip&fenlei=256&rsv_pq=0xc23dafcc00076e78&rsv_t=6743gNBuwGYWrgBnSC7Yl62e52x3CKQWYiI10NeKs73cFjFpwmqJH%2FOI%2FSRG&rqlang=en&rsv_dl=tb&rsv_enter=1&rsv_sug3=5&rsv_sug1=5&rsv_sug7=101&rsv_sug2=0&rsv_btype=i&prefixsug=ip&rsp=4&inputT=2165&rsv_sug4=2719")
			resp, err := client.Do(req)
			if err != nil {
				// fmt.Printf("%v: %v\n", proxyAddr, err)
				// fmt.Printf("+++++++代理不可用：%v+++++++\n", proxyAddr)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				// fmt.Printf("%v: %v\n", proxyAddr, err)
				return
			}
			stringBody := string(body)
			if !isOpenGeolocateSwitch {
				if !strings.Contains(stringBody, checkRspKeywords) {
					return
				}
			} else {
				//直接循环要排除的关键字，任一命中就返回
				for _, keyword := range checkGeolocateConfig.ExcludeKeywords {
					if strings.Contains(stringBody, keyword) {
						// fmt.Println("忽略：" + proxyAddr + "包含：" + keyword.(string))
						return
					}
				}
				//直接循环要必须包含的关键字，任一未命中就返回
				for _, keyword := range checkGeolocateConfig.IncludeKeywords {
					if !strings.Contains(stringBody, keyword) {
						// fmt.Println("忽略：" + proxyAddr + "未包含：" + keyword.(string))
						return
					}
				}

			}
			tmpMu.Lock()
			tmpEffectiveList = append(tmpEffectiveList, proxyAddr)
			tmpMu.Unlock()
		}(proxyAddr)
	}
	Wg.Wait()
	mu.Lock()
	EffectiveList = make([]string, len(tmpEffectiveList))
	copy(EffectiveList, tmpEffectiveList)
	if len(tmpEffectiveList) > 0 {
		proxyIndex = rand.Intn(len(tmpEffectiveList))
	}
	mu.Unlock()
	// 初始化/更新代理统计信息
	InitProxyStats(EffectiveList)
	sec := int(time.Since(startTime).Seconds())
	if sec == 0 {
		sec = 1
	}
	fmt.Printf(ColorGreen+"\n根据配置规则检测完成,用时 [ %vs ] ,共发现 [ %v ] 个可用\n"+ColorReset, sec, len(tmpEffectiveList))
}

func WriteLinesToFile() error {
	file, err := os.Create(LastDataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range EffectiveList {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func DefineDial(ctx context.Context, network, address string) (net.Conn, error) {

	return transmitReqFromClient(network, address)
}

func transmitReqFromClient(network string, address string) (net.Conn, error) {
	// 活跃连接+1（用于优雅关闭）
	mu.Lock()
	ActiveConns++
	mu.Unlock()

	defer func() {
		mu.Lock()
		ActiveConns--
		mu.Unlock()
	}()

	for {
		tempProxy := getNextProxy()
		if tempProxy == "" {
			fmt.Println(ColorRed + "[错误] 已无可用代理，请重新获取代理并运行程序" + ColorReset)
			return nil, fmt.Errorf("no available proxy")
		}
		// 根据日志级别决定是否打印当前使用的代理
		if LogLevel == "debug" {
			fmt.Println(time.Now().Format("2006-01-02 15:04:05") + "\t" + tempProxy)
		}
		timeout := time.Duration(Timeout) * time.Second

		dialer := &net.Dialer{
			Timeout: timeout,
		}

		startTime := time.Now()
		dialect, err := proxy.SOCKS5(network, tempProxy, nil, dialer)
		if err != nil {
			// delInvalidProxy 内部原子化记录失败并判断是否移除
			if delInvalidProxy(tempProxy) {
				fmt.Printf(ColorYellow+"[%s] 代理 %s 连续失败已达上限，已移除，自动切换下一个..."+ColorReset+"\n", time.Now().Format("2006-01-02 15:04:05"), tempProxy)
			} else {
				fmt.Printf(ColorYellow+"[%s] 代理 %s 连接失败，连续失败 %d/%d"+ColorReset+"\n", time.Now().Format("2006-01-02 15:04:05"), tempProxy, getFailStreak(tempProxy), getMaxFailCount())
			}
			continue
		}
		conn, err := dialect.Dial(network, address)
		if err != nil {
			if delInvalidProxy(tempProxy) {
				fmt.Printf(ColorYellow+"[%s] 代理 %s 连接目标失败已达上限，已移除，自动切换下一个..."+ColorReset+"\n", time.Now().Format("2006-01-02 15:04:05"), tempProxy)
			} else {
				fmt.Printf(ColorYellow+"[%s] 代理 %s 连接目标失败，连续失败 %d/%d"+ColorReset+"\n", time.Now().Format("2006-01-02 15:04:05"), tempProxy, getFailStreak(tempProxy), getMaxFailCount())
			}
			continue
		}

		// 记录成功
		respTime := time.Since(startTime)
		RecordProxySuccess(tempProxy, respTime)
		return conn, nil
	}
}

// getFailStreak 获取某代理当前连续失败次数（线程安全）
func getFailStreak(proxyAddr string) int {
	mu.Lock()
	defer mu.Unlock()
	if StatsMap == nil {
		return 0
	}
	s, ok := StatsMap[proxyAddr]
	if !ok {
		return 0
	}
	return s.FailStreak
}

// getMaxFailCount 获取配置的最大失败次数
func getMaxFailCount() int {
	maxFail := MaxFailCount
	if maxFail <= 0 {
		return 3
	}
	return maxFail
}

func getNextProxy() string {
	mu.Lock()
	defer mu.Unlock()

	// 检查是否正在关闭
	select {
	case <-ShutdownChan:
		return "" // 正在关闭，不返回代理
	default:
	}

	if len(EffectiveList) == 0 {
		return "" // 返回空字符串，由调用方处理
	}
	if len(EffectiveList) <= 2 {
		fmt.Printf(ColorYellow+"***可用代理已仅剩%v个,%v,***"+ColorReset+"\n", len(EffectiveList), EffectiveList)
	}
	// 随机选择一个代理，避免短时间内重复
	return EffectiveList[rand.Intn(len(EffectiveList))]
}

// delInvalidProxy 记录一次失败并尝试移除代理（原子操作）
// 返回 true 表示代理已被移除，false 表示连续失败未达上限暂不移除
func delInvalidProxy(proxy string) bool {
	mu.Lock()
	defer mu.Unlock()

	// 更新统计：失败次数 +1，连续失败 +1
	if StatsMap == nil {
		StatsMap = make(map[string]*ProxyStats)
	}
	s, ok := StatsMap[proxy]
	if !ok {
		s = &ProxyStats{}
		StatsMap[proxy] = s
	}
	s.UseCount++
	s.FailCount++
	s.FailStreak++

	// 判断连续失败是否达上限
	maxFail := MaxFailCount
	if maxFail <= 0 {
		maxFail = 3
	}
	if s.FailStreak < maxFail {
		return false // 未达上限，暂不移除
	}

	// 达上限，从 EffectiveList 移除
	for i, p := range EffectiveList {
		if p == proxy {
			EffectiveList = append(EffectiveList[:i], EffectiveList[i+1:]...)
			break
		}
	}
	// 清理统计信息
	delete(StatsMap, proxy)
	return true
}

// 从本地文件获取，格式为IP:PORT

func GetSocks(config Config) {
	GetSocksFromFile(LastDataFile)
	//从fofa获取
	Wg.Add(1)
	go GetSocksFromFofa(config.FOFA)
	//从hunter获取
	Wg.Add(1)
	go GetSocksFromHunter(config.HUNTER)
	//从quake中取
	Wg.Add(1)
	go GetSocksFromQuake(config.QUAKE)
	Wg.Wait()
	//根据IP:PORT去重，此步骤会存在同IP不同端口的情况，这种情况不再单独过滤，这种情况，最终的出口IP可能不一样
	RemoveDuplicates(&SocksList)
}
