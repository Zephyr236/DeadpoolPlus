package main

import (
	"Deadpool/utils"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/armon/go-socks5"
	"github.com/robfig/cron/v3"
)

func main() {
	utils.Banner()
	fmt.Print(utils.ColorCyan + "By:thinkoaa GitHub:https://github.com/thinkoaa/Deadpool\n" + utils.ColorReset + "\n")

	// 解析命令行参数
	configPath := "config.toml"
	lastDataPath := utils.LastDataFile
	help := false
	collectOnly := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			help = true
		} else if arg == "-c" || arg == "--config" {
			if i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				i++
			}
		} else if arg == "-l" || arg == "--lastdata" {
			if i+1 < len(os.Args) {
				lastDataPath = os.Args[i+1]
				utils.LastDataFile = lastDataPath
				i++
			}
		} else if arg == "--collect-only" {
			collectOnly = true
		}
	}

	if help {
		fmt.Println(utils.ColorCyan + "Deadpool 代理池工具 使用帮助:" + utils.ColorReset)
		fmt.Println(utils.ColorCyan + "  -h, --help          显示此帮助信息" + utils.ColorReset)
		fmt.Println(utils.ColorCyan + "  -c, --config <path> 指定配置文件路径 (默认: config.toml)" + utils.ColorReset)
		fmt.Println(utils.ColorCyan + "  -l, --lastdata <path> 指定lastdata文件路径 (默认: lastData.txt)" + utils.ColorReset)
		fmt.Println(utils.ColorCyan + "  --collect-only       仅采集+检测代理后退出，不启动SOCKS5服务" + utils.ColorReset)
		os.Exit(0)
	}

	// 读取配置文件
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		fmt.Printf(utils.ColorRed+"配置文件 %s 存在错误: %v\n"+utils.ColorReset, configPath, err)
		fmt.Println(utils.ColorRed + "请检查配置文件格式是否正确，参考 README 中的配置说明" + utils.ColorReset)
		os.Exit(1)
	}

	// 设置日志级别
	utils.LogLevel = strings.TrimSpace(config.Listener.LogLevel)
	if utils.LogLevel == "" {
		utils.LogLevel = "normal"
	}

	// 初始化代理统计、优雅关闭通道、最大失败次数
	utils.StatsMap = make(map[string]*utils.ProxyStats)
	utils.ShutdownChan = make(chan struct{})
	utils.MaxFailCount = config.CheckSocks.MaxFailCount

	// 从本地文件中取socks代理
	if utils.LogLevel == "debug" {
		fmt.Print(utils.ColorCyan + "***debug模式: 每个请求的代理信息会打印到命令行***\n" + utils.ColorReset + "\n")
	}
	fmt.Print(utils.ColorYellow + "***直接使用fmt打印信息，基本上是打印异常的信息***\n" + utils.ColorReset + "\n")

	utils.Timeout = config.CheckSocks.Timeout

	// 快速启动：如果 lastData.txt 存在，直接加载已验证代理，立即启动 SOCKS5 服务
	// 新代理的采集和检测在后台异步进行
	quickStart := false
	existingCount := utils.InitEffectiveFromFile(utils.LastDataFile)
	if existingCount > 0 {
		quickStart = true
		fmt.Printf(utils.ColorGreen+"从 %s 加载了 %d 个已验证代理，立即启动服务\n"+utils.ColorReset, utils.LastDataFile, existingCount)
	}

	if !quickStart {
		// 传统模式：先采集+检测所有代理，再启动服务
		if lastDataPath == utils.LastDataFile {
			utils.GetSocks(config)
		}
		if len(utils.SocksList) == 0 {
			fmt.Print(utils.ColorRed + "未发现代理数据,请调整配置信息,或向" + utils.LastDataFile + "中直接写入IP:PORT格式的代理\n程序退出" + utils.ColorReset + "\n")
			os.Exit(1)
		}
		fmt.Printf(utils.ColorCyan+"根据IP:PORT去重后，共发现%v个代理\n检测可用性中......\n"+utils.ColorReset, len(utils.SocksList))
		utils.CheckProxy(config.CheckSocks, utils.SocksList)
	} else {
		// 快速启动模式：后台采集+检测新代理，服务立即可用
		if lastDataPath == utils.LastDataFile {
			go func() {
				utils.GetSocks(config)
				if len(utils.SocksList) > 0 {
					fmt.Printf(utils.ColorCyan+"\n根据IP:PORT去重后，共发现%v个新代理，后台检测中...\n"+utils.ColorReset, len(utils.SocksList))
					utils.CheckProxy(config.CheckSocks, utils.SocksList)
					utils.WriteLinesToFile()
					fmt.Print(utils.ColorGreen + "\n*** 后台代理检测完成，有效代理池已更新 ***\n" + utils.ColorReset)
				}
			}()
		}
	}
	//根据配置，定时检测内存中的代理存活信息
	cron := cron.New()
	periodicChecking := strings.TrimSpace(config.Task.PeriodicChecking)
	cronFlag := false
	if periodicChecking != "" {
		cronFlag = true
		cron.AddFunc(periodicChecking, func() {
			fmt.Printf(utils.ColorBlue + "\n===代理存活自检 开始===\n\n" + utils.ColorReset)
			tempList := make([]string, len(utils.EffectiveList))
			copy(tempList, utils.EffectiveList)
			utils.CheckProxy(config.CheckSocks, tempList)
			fmt.Printf(utils.ColorBlue + "\n===代理存活自检 结束===\n\n" + utils.ColorReset)
		})
	}
	//根据配置信息，周期性取本地以及fofa的数据
	periodicGetSocks := strings.TrimSpace(config.Task.PeriodicGetSocks)
	if periodicGetSocks != "" {
		cronFlag = true
		cron.AddFunc(periodicGetSocks, func() {
			fmt.Printf(utils.ColorBlue + "\n===周期性取代理数据 开始===\n\n" + utils.ColorReset)
			utils.SocksList = utils.SocksList[:0]
			utils.GetSocks(config)
			fmt.Printf(utils.ColorCyan+"根据IP:PORT去重后，共发现%v个代理\n检测可用性中......\n"+utils.ColorReset, len(utils.SocksList))
			utils.CheckProxy(config.CheckSocks, utils.SocksList)
			if len(utils.EffectiveList) != 0 {
				utils.WriteLinesToFile() //存活代理写入硬盘，以备下次启动直接读取
			}
			fmt.Printf(utils.ColorBlue + "\n===周期性取代理数据 结束===\n\n" + utils.ColorReset)

		})
	}

	if cronFlag {
		cron.Start()
	}

	if len(utils.EffectiveList) == 0 {
		fmt.Println(utils.ColorRed + "根据规则检测后，未发现满足要求的代理,请调整配置,程序退出" + utils.ColorReset)
		os.Exit(1)
	}

	utils.WriteLinesToFile() //有效代理写入硬盘，以备下次启动直接读取

	if collectOnly {
		fmt.Printf(utils.ColorGreen+"采集完成！%d 个可用代理已保存至 %s\n"+utils.ColorReset, len(utils.EffectiveList), utils.LastDataFile)
		return
	}

	// 开启监听
	conf := &socks5.Config{
		Dial:   utils.DefineDial,
		Logger: log.New(io.Discard, "", log.LstdFlags),
	}
	userName := strings.TrimSpace(config.Listener.UserName)
	password := strings.TrimSpace(config.Listener.Password)
	if userName != "" && password != "" {
		cator := socks5.UserPassAuthenticator{Credentials: socks5.StaticCredentials{
			userName: password,
		}}
		conf.AuthMethods = []socks5.Authenticator{cator}
	}
	server, _ := socks5.New(conf)
	listenerAddr := config.Listener.IP + ":" + strconv.Itoa(config.Listener.Port)
	fmt.Printf(utils.ColorGreen+"======其他工具通过配置 socks5://%v 使用收集的代理,如有账号密码，记得配置======\n"+utils.ColorReset, listenerAddr)
	fmt.Println(utils.ColorYellow + "按回车键随机切换到下一个代理IP，输入 s 回车查看统计..." + utils.ColorReset)

	// 手动创建 listener，支持优雅关闭
	l, err := net.Listen("tcp", listenerAddr)
	if err != nil {
		fmt.Printf(utils.ColorRed+"本地监听服务启动失败：%v\n"+utils.ColorReset, err)
		os.Exit(1)
	}

	// 文件触发统计：创建 .dump_stats 文件时打印代理统计
	go func() {
		triggerFile := ".dump_stats"
		for {
			if _, err := os.Stat(triggerFile); err == nil {
				os.Remove(triggerFile)
				utils.PrintStats()
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// 信号监听 goroutine：捕获 SIGINT/SIGTERM，执行优雅关闭
	go func(listener net.Listener) {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println(utils.ColorCyan + "\n\n[优雅关闭] 收到关闭信号，准备退出..." + utils.ColorReset)
		fmt.Println(utils.ColorYellow + "[提示] 再次按 Ctrl+C 可强制退出" + utils.ColorReset)
		// 关闭 ShutdownChan，通知拒绝新连接
		select {
		case <-utils.ShutdownChan:
			// 已经关闭
		default:
			close(utils.ShutdownChan)
		}
		// 打印统计信息
		utils.PrintStats()
		// 关闭 listener，停止接受新连接
		listener.Close()
		// 等待活跃连接完成（最多等待30秒）
		timeout := time.After(30 * time.Second)
		for {
			select {
			case <-sigChan:
				// 再次收到关闭信号，强制退出
				fmt.Println(utils.ColorRed + "\n[优雅关闭] 收到强制退出信号，立即退出！" + utils.ColorReset)
				os.Exit(0)
			case <-timeout:
				fmt.Println(utils.ColorYellow + "[优雅关闭] 等待超时，强制退出" + utils.ColorReset)
				os.Exit(0)
			default:
				active := utils.GetActiveConns()
				if active == 0 {
					fmt.Println(utils.ColorGreen + "[优雅关闭] 所有连接已完成，退出。" + utils.ColorReset)
					os.Exit(0)
				}
				fmt.Printf(utils.ColorCyan+"[优雅关闭] 等待 %d 个活跃连接完成...\n"+utils.ColorReset, active)
				time.Sleep(1 * time.Second)
			}
		}
	}(l)

	// 使用goroutine监听键盘输入（支持 s 查看统计）
	go func() {
		for {
			var input string
			fmt.Scanln(&input)
			input = strings.TrimSpace(input)
			if input == "s" || input == "S" {
				utils.PrintStats()
				fmt.Println(utils.ColorYellow + "按回车键随机切换到下一个代理IP，输入 s 回车查看统计..." + utils.ColorReset)
				continue
			}
			addr, total := utils.SwitchAndGetProxy()
			if addr != "" {
				fmt.Printf(utils.ColorGreen+"已随机切换到代理IP: %s (剩余可用: %d)\n"+utils.ColorReset, addr, total)
			} else {
				fmt.Println(utils.ColorRed + "没有可用的代理IP" + utils.ColorReset)
			}
			fmt.Println(utils.ColorYellow + "按回车键随机切换到下一个代理IP，输入 s 回车查看统计..." + utils.ColorReset)
		}
	}()

	// 启动 SOCKS5 服务（阻塞，直到 listener 被关闭）
	if err := server.Serve(l); err != nil {
		// 如果是关闭导致的错误（listener 已关闭），正常退出
		select {
		case <-utils.ShutdownChan:
			// 正常关闭，等待信号处理的 goroutine 完成退出
			// 主 goroutine 阻塞等待（信号处理 goroutine 会调用 os.Exit）
			select {}
		default:
			fmt.Printf(utils.ColorRed+"SOCKS5 服务异常: %v\n"+utils.ColorReset, err)
			os.Exit(1)
		}
	}

}
