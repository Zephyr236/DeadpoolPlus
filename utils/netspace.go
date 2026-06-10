package utils

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 从quake获取，结果为 socks5://IP:PORT
func GetSocksFromQuake(quake QUAKEConfig) {
	defer Wg.Done()
	if quake.Switch != "open" {
		fmt.Println("---未开启quake---")
		return
	}
	fmt.Printf("***已开启quake,将根据配置条件从quake中获取%d条数据，然后进行有效性检测***\n", quake.ResultSize)
	jsonCondition := "{\"query\": \"" + strings.Replace(quake.QueryString, `"`, `\"`, -1) + "\",\"latest\":\"True\",\"start\": 0,\"size\": " + strconv.Itoa(quake.ResultSize) + ",\"include\":[\"ip\",\"port\"]}"
	headers := map[string]string{
		"X-QuakeToken": quake.Key,
		"Content-Type": "application/json"}
	content, err := fetchContent(quake.APIURL, "POST", 60, nil, headers, jsonCondition)
	if err != nil {
		fmt.Println("quake异常", err)
		return
	}
	var data map[string]interface{}
	json.Unmarshal([]byte(content), &data)
	code, _ := strconv.ParseFloat("0", 64)
	if data["code"] != code {
		fmt.Println("QUAKE:", data["message"])
		return
	}
	arr, ok := data["data"].([]interface{})
	if !ok {
		fmt.Println("quake: 返回数据格式异常")
		return
	}
	fmt.Println("+++quake数据已取+++")
	for _, item := range arr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ip, ok1 := itemMap["ip"].(string)
		port, ok2 := itemMap["port"].(float64)
		if !ok1 || !ok2 {
			continue
		}
		addSocks("socks5://" + ip + ":" + strconv.FormatFloat(port, 'f', -1, 64))
	}
}

// 从FOFA获取，结果为 protocol://IP:PORT
func GetSocksFromFofa(fofa FOFAConfig) {
	defer Wg.Done()
	if fofa.Switch != "open" {
		fmt.Println("---未开启fofa---")
		return
	}

	totalSocks5 := len(fofa.QueryStrings)
	fmt.Printf("***已开启fofa, SOCKS5 查询 %d 条，每条最多获取 %d 条数据***\n", totalSocks5, fofa.ResultSize)

	totalCollected := 0
	for i, qs := range fofa.QueryStrings {
		qs = strings.TrimSpace(qs)
		if qs == "" {
			continue
		}
		fmt.Printf("[FOFA-SOCKS5 %d/%d] 查询: %s\n", i+1, totalSocks5, qs)
		params := map[string]string{
			"email":   fofa.Email,
			"key":     fofa.Key,
			"fields":  "ip,port",
			"qbase64": base64.URLEncoding.EncodeToString([]byte(qs)),
			"size":    strconv.Itoa(fofa.ResultSize)}
		content, err := fetchContent(fofa.APIURL, "GET", 60, params, nil, "")
		if err != nil {
			fmt.Printf("FOFA-SOCKS5 查询 [%s] 异常: %v\n", qs, err)
			continue
		}
		var data map[string]interface{}
		json.Unmarshal([]byte(content), &data)
		if data["error"] == true {
			fmt.Println("FOFA:", data["errmsg"])
			continue
		}
		array, ok := data["results"].([]interface{})
		if !ok {
			fmt.Println("FOFA: 返回数据格式异常")
			continue
		}
		count := 0
		for _, itemArray := range array {
			itemSlice, ok := itemArray.([]interface{})
			if !ok || len(itemSlice) < 2 {
				continue
			}
			ip, ok1 := itemSlice[0].(string)
			port, ok2 := itemSlice[1].(string)
			if !ok1 || !ok2 {
				continue
			}
			addSocks("socks5://" + ip + ":" + port)
			count++
		}
		totalCollected += count
		fmt.Printf("+++FOFA-SOCKS5 查询完成，获取 %d 条+++\n", count)
		if i < totalSocks5-1 {
			time.Sleep(2 * time.Second)
		}
	}
	fmt.Printf("+++fofa SOCKS5 查询完成，共获取 %d 条+++\n", totalCollected)
}

// GetProxiesFromPools 从 FOFA 搜索公开代理池并爬取已维护好的代理
func GetProxiesFromPools(fofa FOFAConfig) {
	defer Wg.Done()
	if fofa.Switch != "open" || fofa.PoolQuery == "" {
		return
	}

	fmt.Printf("***搜索代理池: %s***\n", fofa.PoolQuery)
	qs := fofa.PoolQuery
	params := map[string]string{
		"email":   fofa.Email,
		"key":     fofa.Key,
		"fields":  "ip,port",
		"qbase64": base64.URLEncoding.EncodeToString([]byte(qs)),
		"size":    strconv.Itoa(fofa.PoolResultSize)}
	content, err := fetchContent(fofa.APIURL, "GET", 60, params, nil, "")
	if err != nil {
		fmt.Printf("代理池搜索异常: %v\n", err)
		return
	}
	var data map[string]interface{}
	json.Unmarshal([]byte(content), &data)
	if data["error"] == true {
		fmt.Println("FOFA 代理池搜索:", data["errmsg"])
		return
	}
	pools, ok := data["results"].([]interface{})
	if !ok || len(pools) == 0 {
		fmt.Println("未发现代理池服务")
		return
	}
	fmt.Printf("=== FOFA 发现 %d 个代理池，开始并发爬取 ===\n", len(pools))

	poolConcurrency := 50 // 并发爬取数
	sem := make(chan struct{}, poolConcurrency)
	var poolMu sync.Mutex
	var poolWg sync.WaitGroup
	totalProxies := 0

	for idx, pool := range pools {
		arr, ok := pool.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}
		ip, _ := arr[0].(string)
		port, _ := arr[1].(string)

		poolWg.Add(1)
		sem <- struct{}{}
		go func(idx int, ip, port string) {
			defer poolWg.Done()
			defer func() { <-sem }()

			// 爬取 /all 接口
			allURL := "http://" + ip + ":" + port + "/all"
			body, err := fetchContent(allURL, "GET", 8, nil, nil, "")
			if err != nil {
				return
			}

			var proxies []struct {
				Proxy      string `json:"proxy"`
				LastStatus bool   `json:"last_status"`
			}
			if err := json.Unmarshal([]byte(body), &proxies); err != nil {
				return
			}

			count := 0
			for _, p := range proxies {
				if p.LastStatus {
					for _, proto := range []string{"socks5", "socks4", "http", "https"} {
						addSocks(proto + "://" + p.Proxy)
					}
					count++
				}
			}
			poolMu.Lock()
			totalProxies += count
			poolMu.Unlock()
			if count > 0 {
				fmt.Printf("[Pool %d/%d] %s:%s → %d 代理\n", idx+1, len(pools), ip, port, count)
			}
		}(idx, ip, port)
	}
	poolWg.Wait()
	fmt.Printf("=== 代理池爬取完成，共获取 %d 个代理 ===\n", totalProxies)
}

// GetProxiesFromURLs 从公开代理列表 URL 获取代理
func GetProxiesFromURLs(urls []string) {
	defer Wg.Done()
	if len(urls) == 0 {
		return
	}

	var urlWg sync.WaitGroup
	sem := make(chan struct{}, 10) // 最多 10 个并发下载
	totalCollected := 0
	var totalMu sync.Mutex

	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}

		urlWg.Add(1)
		sem <- struct{}{}
		go func(url string) {
			defer urlWg.Done()
			defer func() { <-sem }()

			fmt.Printf("***下载代理列表: %s***\n", url)
			content, err := fetchContent(url, "GET", 30, nil, nil, "")
			if err != nil {
				fmt.Printf("  下载失败: %v\n", err)
				return
			}

			count := 0
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if strings.Contains(line, "://") {
					// 有协议前缀，直接使用
					addSocks(line)
					count++
				} else {
					// 无协议前缀，生成所有协议变体
					for _, proto := range []string{"socks5", "socks4", "http", "https"} {
						addSocks(proto + "://" + line)
					}
					count++
				}
			}
			totalMu.Lock()
			totalCollected += count
			totalMu.Unlock()
			fmt.Printf("  %s → %d 个代理\n", url, count)
		}(u)
	}
	urlWg.Wait()
	fmt.Printf("=== 公开列表下载完成，共获取 %d 个代理 ===\n", totalCollected)
}

// 从鹰图获取，结果为IP:PORT
func GetSocksFromHunter(hunter HUNTERConfig) {
	defer Wg.Done()
	if hunter.Switch != "open" {
		fmt.Println("---未开启hunter---")
		return
	}
	fmt.Printf("***已开启hunter,将根据配置条件从hunter中获取%d条数据,然后进行有效性检测***\n", hunter.ResultSize)

	var exeData int //记录处理了几条
	end := hunter.ResultSize / 100
	for i := 1; i <= end; i++ {
		params := map[string]string{
			"api-key":   hunter.Key,
			"search":    base64.URLEncoding.EncodeToString([]byte(hunter.QueryString)),
			"page":      strconv.Itoa(i),
			"page_size": "100"}
		fmt.Printf("HUNTER:每页100条,正在查询第%v页\n", i)
		content, err := fetchContent(hunter.APIURL, "GET", 60, params, nil, "")
		if err != nil {
			fmt.Println("访问hunter异常", err)
			return
		}
		var data map[string]interface{}
		json.Unmarshal([]byte(content), &data)
		code, _ := strconv.ParseFloat("200", 64)
		if data["code"] != code {
			fmt.Println("HUNTER:", data["message"])
			return
		}

		rsData, ok := data["data"].(map[string]interface{})
		if !ok {
			fmt.Println("HUNTER: 返回数据格式异常")
			return
		}
		total, ok := rsData["total"].(float64)
		if !ok {
			fmt.Println("HUNTER: 返回total字段格式异常")
			break
		}
		if total == 0 {
			fmt.Println("HUNTER:xxx根据配置语法,未取到数据xxx")
			break
		}
		arr, ok := rsData["arr"].([]interface{})
		if !ok {
			fmt.Println("HUNTER: 返回arr字段格式异常")
			break
		}
		for _, item := range arr {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			ip, ok1 := itemMap["ip"].(string)
			port, ok2 := itemMap["port"].(float64)
			if !ok1 || !ok2 {
				continue
			}
			exeData++
			addSocks("socks5://" + ip + ":" + strconv.FormatFloat(port, 'f', -1, 64))
		}
		if float64(exeData) >= total {
			break
		}
		if end > 1 && i != end {
			time.Sleep(3 * time.Second) //防止hunter提示访问过快获取不到结果
		}
	}
	fmt.Println("+++hunter数据已取+++")
}

// 从本地文件获取，格式为 protocol://IP:PORT（如 socks5://1.2.3.4:1080、http://5.6.7.8:8080）
func GetSocksFromFile(socksFileName string) {
	_, err := os.Stat(socksFileName)
	if !os.IsNotExist(err) {
		fmt.Println("***当前目录下存在" + socksFileName + ",将按行读取格式为 protocol://IP:PORT 的代理***")
		file, err := os.Open(socksFileName)
		if err != nil {
			fmt.Println("读取文件"+socksFileName+"异常，略过该文件中的代理，异常信息为:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				addSocks(line) // 使用线程安全的 addSocks，避免并发写入冲突
			}
		}
		// 检查扫描过程中是否发生了错误
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file,请确认文件中的代理是 protocol://IP:PORT 格式（如 socks5://1.2.3.4:1080）:", err)
		}
	} else {
		fmt.Println(socksFileName + "文件不存在，将根据配置信息从网络空间测绘平台取代理")
	}
}
