package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/judwhite/go-svc"
	"github.com/mreiferson/go-options"
	"github.com/nsqio/nsq/internal/lg"
	"github.com/nsqio/nsq/internal/version"
	"github.com/nsqio/nsq/nsqd"
)

type program struct {
	once sync.Once
	nsqd *nsqd.NSQD
}

func main() {
	prg := &program{}
	if err := svc.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		logFatal("%s", err)
	}
}

// Init
// 1 构造 Options
// 2 初始化 NSQD
func (p *program) Init(env svc.Environment) error {
	// 设置默认值
	opts := nsqd.NewOptions()

	/*flag解析命令行参数*/

	// 先设置默认值
	flagSet := nsqdFlagSet(opts)
	// 解析命令行参数
	flagSet.Parse(os.Args[1:])

	// 使用时间作为随机种子值
	rand.Seed(time.Now().UTC().UnixNano())

	// 遇到命令 nsqd -version。 用于打印版本，然后退出
	if flagSet.Lookup("version").Value.(flag.Getter).Get().(bool) {
		fmt.Println(version.String("nsqd"))
		os.Exit(0)
	}

	// 处理配置信息
	var cfg config
	// 如果传入 --config='/xxx/xxx.toml'
	configFile := flagSet.Lookup("config").Value.String()
	if configFile != "" {
		_, err := toml.DecodeFile(configFile, &cfg)
		if err != nil {
			logFatal("failed to load config file %s - %s", configFile, err)
		}
	}
	// 验证一下。有错误会直接停止进程
	cfg.Validate()
	// 将 flagSet 和 config 中的配置信息合并到 opts
	options.Resolve(opts, flagSet, cfg)

	// 创建 nsqd 【重点方法】
	nsqd, err := nsqd.New(opts)
	if err != nil {
		logFatal("failed to instantiate nsqd - %s", err)
	}
	p.nsqd = nsqd

	return nil
}

func (p *program) Start() error {
	// 加载元数据
	err := p.nsqd.LoadMetadata()
	if err != nil {
		logFatal("failed to load metadata - %s", err)
	}
	// 持久化元数据
	err = p.nsqd.PersistMetadata()
	if err != nil {
		logFatal("failed to persist metadata - %s", err)
	}

	go func() {
		// 启动 nsqd
		err := p.nsqd.Main()
		if err != nil {
			p.Stop()
			os.Exit(1)
		}
	}()

	return nil
}

func (p *program) Stop() error {
	p.once.Do(func() {
		p.nsqd.Exit()
	})
	return nil
}

func (p *program) Handle(s os.Signal) error {
	return svc.ErrStop
}

// Context returns a context that will be canceled when nsqd initiates the shutdown
func (p *program) Context() context.Context {
	return p.nsqd.Context()
}

func logFatal(f string, args ...interface{}) {
	lg.LogFatal("[nsqd] ", f, args...)
}
