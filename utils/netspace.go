package utils

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	totalHTTP := len(fofa.HTTPQueryStrings)
	fmt.Printf("***已开启fofa, SOCKS5 查询 %d 条 + HTTP 查询 %d 条，每条最多获取 %d 条数据***\n", totalSocks5, totalHTTP, fofa.ResultSize)

	totalCollected := 0

	// 交替执行 SOCKS5 和 HTTP 查询，避免连续同类查询触发 API 限流
	maxLen := totalSocks5
	if totalHTTP > maxLen {
		maxLen = totalHTTP
	}
	for i := 0; i < maxLen; i++ {
		// SOCKS5 查询
		if i < totalSocks5 {
			qs := strings.TrimSpace(fofa.QueryStrings[i])
			if qs != "" {
				fmt.Printf("[FOFA-SOCKS5 %d/%d] 查询: %s\n", i+1, totalSocks5, qs)
				count := fofaSingleQuery(fofa, qs, "socks5", "SOCKS5")
				totalCollected += count
			}
			time.Sleep(2 * time.Second)
		}
		// HTTP 查询
		if i < totalHTTP {
			qs := strings.TrimSpace(fofa.HTTPQueryStrings[i])
			if qs != "" {
				fmt.Printf("[FOFA-HTTP %d/%d] 查询: %s\n", i+1, totalHTTP, qs)
				count := fofaSingleQuery(fofa, qs, "http", "HTTP")
				totalCollected += count
			}
			time.Sleep(2 * time.Second)
		}
	}

	fmt.Printf("+++fofa全部查询完成，共获取 %d 条数据（SOCKS5 + HTTP）+++\n", totalCollected)
}

// fofaSingleQuery 执行单条 FOFA 查询，支持超时重试，返回获取的代理数
func fofaSingleQuery(fofa FOFAConfig, qs string, protocol string, label string) int {
	for attempt := 1; attempt <= 2; attempt++ {
		params := map[string]string{
			"email":   fofa.Email,
			"key":     fofa.Key,
			"fields":  "ip,port",
			"qbase64": base64.URLEncoding.EncodeToString([]byte(qs)),
			"size":    strconv.Itoa(fofa.ResultSize)}
		content, err := fetchContent(fofa.APIURL, "GET", 60, params, nil, "")
		if err != nil {
			if attempt == 1 {
				fmt.Printf("FOFA-%s 查询超时，3秒后重试...\n", label)
				time.Sleep(3 * time.Second)
				continue
			}
			fmt.Printf("FOFA-%s 查询 [%s] 异常: %v\n", label, qs, err)
			return 0
		}
		var data map[string]interface{}
		json.Unmarshal([]byte(content), &data)
		if data["error"] == true {
			fmt.Println("FOFA:", data["errmsg"])
			return 0
		}
		array, ok := data["results"].([]interface{})
		if !ok {
			fmt.Println("FOFA: 返回数据格式异常")
			return 0
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
			addSocks(protocol + "://" + ip + ":" + port)
			count++
		}
		fmt.Printf("+++FOFA-%s 查询完成，获取 %d 条+++\n", label, count)
		return count
	}
	return 0
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
