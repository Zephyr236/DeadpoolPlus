package main

import (
	"Deadpool/utils"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/armon/go-socks5"
	"github.com/robfig/cron/v3"
)

func main() {
	utils.Banner()
	fmt.Print("By:thinkoaa GitHub:https://github.com/thinkoaa/Deadpool\n\n\n")

	// 解析命令行参数
	configPath := "config.toml"
	lastDataPath := utils.LastDataFile
	help := false

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
		}
	}

	if help {
		fmt.Println("Deadpool 代理池工具 使用帮助:")
		fmt.Println("  -h, --help          显示此帮助信息")
		fmt.Println("  -c, --config <path> 指定配置文件路径 (默认: config.toml)")
		fmt.Println("  -l, --lastdata <path> 指定lastdata文件路径 (默认: lastData.txt)")
		fmt.Println("                      使用此选项时，不会重新从网络空间获取代理")
		os.Exit(0)
	}

	// 读取配置文件
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("配置文件 %s 存在错误: %v\n", configPath, err)
		fmt.Println("请检查配置文件格式是否正确，参考 README 中的配置说明")
		os.Exit(1)
	}

	// 设置日志级别
	utils.LogLevel = strings.TrimSpace(config.Listener.LogLevel)
	if utils.LogLevel == "" {
		utils.LogLevel = "normal"
	}

	// 从本地文件中取socks代理
	if utils.LogLevel == "debug" {
		fmt.Print("***debug模式: 每个请求的代理信息会打印到命令行***\n\n")
	}
	fmt.Print("***直接使用fmt打印当前使用的代理,若高并发时,命令行打印可能会阻塞，不对打印做特殊处理，可忽略，不会影响实际的请求转发***\n\n")
	if lastDataPath == utils.LastDataFile {
		// 未指定自定义lastdata路径时，从网络空间获取代理
		utils.GetSocks(config)
	}

	if len(utils.SocksList) == 0 {
		fmt.Println("未发现代理数据,请调整配置信息,或向" + utils.LastDataFile + "中直接写入IP:PORT格式的socks5代理\n程序退出")
		os.Exit(1)
	}
	fmt.Printf("根据IP:PORT去重后，共发现%v个代理\n检测可用性中......\n", len(utils.SocksList))

	//开始检测代理存活性

	utils.Timeout = config.CheckSocks.Timeout
	utils.CheckSocks(config.CheckSocks, utils.SocksList)
	//根据配置，定时检测内存中的代理存活信息
	cron := cron.New()
	periodicChecking := strings.TrimSpace(config.Task.PeriodicChecking)
	cronFlag := false
	if periodicChecking != "" {
		cronFlag = true
		cron.AddFunc(periodicChecking, func() {
			fmt.Printf("\n===代理存活自检 开始===\n\n")
			tempList := make([]string, len(utils.EffectiveList))
			copy(tempList, utils.EffectiveList)
			utils.CheckSocks(config.CheckSocks, tempList)
			fmt.Printf("\n===代理存活自检 结束===\n\n")
		})
	}
	//根据配置信息，周期性取本地以及hunter、quake、fofa的数据
	periodicGetSocks := strings.TrimSpace(config.Task.PeriodicGetSocks)
	if periodicGetSocks != "" {
		cronFlag = true
		cron.AddFunc(periodicGetSocks, func() {
			fmt.Printf("\n===周期性取代理数据 开始===\n\n")
			utils.SocksList = utils.SocksList[:0]
			utils.GetSocks(config)
			fmt.Printf("根据IP:PORT去重后，共发现%v个代理\n检测可用性中......\n", len(utils.SocksList))
			utils.CheckSocks(config.CheckSocks, utils.SocksList)
			if len(utils.EffectiveList) != 0 {
				utils.WriteLinesToFile() //存活代理写入硬盘，以备下次启动直接读取
			}
			fmt.Printf("\n===周期性取代理数据 结束===\n\n")

		})
	}

	if cronFlag {
		cron.Start()
	}

	if len(utils.EffectiveList) == 0 {
		fmt.Println("根据规则检测后，未发现满足要求的代理,请调整配置,程序退出")
		os.Exit(1)
	}

	utils.WriteLinesToFile() //存活代理写入硬盘，以备下次启动直接读取

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
	listener := config.Listener.IP + ":" + strconv.Itoa(config.Listener.Port)
	fmt.Printf("======其他工具通过配置 socks5://%v 使用收集的代理,如有账号密码，记得配置======\n", listener)
	fmt.Println("按回车键切换到下一个代理IP...")

	// 使用goroutine监听键盘输入
	go func() {
		for {
			var input string
			fmt.Scanln(&input)
			utils.SetNextProxyIndex()
			currentIndex := utils.GetCurrentProxyIndex()
			if len(utils.EffectiveList) > 0 {
				fmt.Printf("已切换到代理IP: %s (索引: %d/%d)\n", utils.EffectiveList[currentIndex], currentIndex+1, len(utils.EffectiveList))
			} else {
				fmt.Println("没有可用的代理IP")
			}
			fmt.Println("按回车键切换到下一个代理IP...")
		}
	}()

	if err := server.ListenAndServe("tcp", listener); err != nil {
		fmt.Printf("本地监听服务启动失败：%v\n", err)
		os.Exit(1)
	}

}
