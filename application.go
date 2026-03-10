package cherry

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	cconst "github.com/cherry-game/cherry/const"
	ctime "github.com/cherry-game/cherry/extend/time"
	cutils "github.com/cherry-game/cherry/extend/utils"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	cserializer "github.com/cherry-game/cherry/net/serializer"
	cprofile "github.com/cherry-game/cherry/profile"
)

const (
	Cluster    NodeMode = 1 // 集群模式
	Standalone NodeMode = 2 // 单机模式
)

type (
	NodeMode byte

	Application struct {
		cfacade.INode                      //节点信息
		isFrontend    bool                 //是否与前端交互
		nodeMode      NodeMode             //节点模式
		startTime     ctime.CherryTime     // application 启动时间
		running       int32                // 是否为运行状态
		dieChan       chan bool            // 等待结束chan
		onShutdownFn  []func()             // shutdown函数
		components    []cfacade.IComponent // 组件集合
		serializer    cfacade.ISerializer  // 序列化器
		discovery     cfacade.IDiscovery   // 服务注册发现组件
		cluster       cfacade.ICluster     // 集群组件
		actorSystem   *cactor.Component    // actor系统
		netParser     cfacade.INetParser   // 网络包解析器
	}
)

// NewApp 创建新实例(入参为配置)
func NewApp(profileFilePath, nodeID string, isFrontend bool, mode NodeMode) *Application {
	node, err := cprofile.Init(profileFilePath, nodeID)
	if err != nil {
		panic(err)
	}

	return NewAppNode(node, isFrontend, mode)
}

// NewAppNode 创建新实例(入参为Node)
func NewAppNode(node cfacade.INode, isFrontend bool, mode NodeMode) *Application {
	// 设置Logger
	clog.SetNodeLogger(node)

	// 打印LOGO
	clog.Info(cconst.GetLOGO())

	app := &Application{
		INode:       node,
		serializer:  cserializer.NewProtobuf(),
		isFrontend:  isFrontend,
		nodeMode:    mode,
		startTime:   ctime.Now(),
		running:     0,
		dieChan:     make(chan bool),
		actorSystem: cactor.New(),
	}

	return app
}

func (a *Application) IsFrontend() bool {
	return a.isFrontend
}

func (a *Application) NodeMode() NodeMode {
	return a.nodeMode
}

func (a *Application) Running() bool {
	return a.running > 0
}

func (a *Application) DieChan() chan bool {
	return a.dieChan
}

// Register 注册组件
func (a *Application) Register(components ...cfacade.IComponent) {

	//运行状态不允许注册
	if a.Running() {
		return
	}

	for _, c := range components {
		if c == nil || c.Name() == "" {
			clog.Errorf("[component = %T] name is nil", c)
			return
		}
		//根据名字查找组件
		result := a.Find(c.Name())
		//存在了 不允许重复添加
		if result != nil {
			clog.Errorf("[component name = %s] is duplicate.", c.Name())
			return
		}
		//添加组件
		a.components = append(a.components, c)
	}
}

func (a *Application) Find(name string) cfacade.IComponent {
	if name == "" {
		return nil
	}

	for _, component := range a.components {
		if component.Name() == name {
			return component
		}
	}
	return nil
}

// Remove 根据名字移除组件
func (a *Application) Remove(name string) cfacade.IComponent {
	if name == "" {
		return nil
	}

	var removeComponent cfacade.IComponent
	for i := 0; i < len(a.components); i++ {
		if a.components[i].Name() == name {
			removeComponent = a.components[i]
			a.components = append(a.components[:i], a.components[i+1:]...)
			i--
		}
	}
	return removeComponent
}

func (a *Application) All() []cfacade.IComponent {
	return a.components
}

func (a *Application) OnShutdown(fn ...func()) {
	a.onShutdownFn = append(a.onShutdownFn, fn...)
}

// Startup load components before startup
func (a *Application) Startup() {
	defer func() {
		if r := recover(); r != nil {
			clog.Error(r)
		}
	}()

	//已经是running状态
	if a.Running() {
		clog.Error("Application has running.")
		return
	}

	defer func() {
		//刷新日志
		clog.Flush()
	}()

	// 注册actor系统
	a.Register(a.actorSystem)

	// add connector component
	if a.netParser != nil {
		for _, connector := range a.netParser.Connectors() {
			a.Register(connector)
		}
	}

	clog.Info("-------------------------------------------------")
	clog.Infof("[nodeID      = %s] application is starting...", a.NodeID())
	clog.Infof("[nodeType    = %s]", a.NodeType())
	clog.Infof("[pid         = %d]", os.Getpid())
	clog.Infof("[startTime   = %s]", a.StartTime())
	clog.Infof("[profilePath = %s]", cprofile.Path())
	clog.Infof("[profileName = %s]", cprofile.Name())
	clog.Infof("[env         = %s]", cprofile.Env())
	clog.Infof("[debug       = %v]", cprofile.Debug())
	clog.Infof("[printLevel  = %s]", cprofile.PrintLevel())
	clog.Infof("[logLevel    = %s]", clog.DefaultLogger.LogLevel)
	clog.Infof("[stackLevel  = %s]", clog.DefaultLogger.StackLevel)
	clog.Infof("[writeFile   = %v]", clog.DefaultLogger.EnableWriteFile)
	clog.Infof("[serializer  = %s]", a.serializer.Name())
	clog.Info("-------------------------------------------------")

	// 所有组件设置App引用
	for _, c := range a.components {
		c.Set(a)
		clog.Infof("[component = %s] is added.", c.Name())
	}
	clog.Info("-------------------------------------------------")

	// 执行Init
	for _, c := range a.components {
		clog.Infof("[component = %s] -> OnInit().", c.Name())
		c.Init()
	}
	clog.Info("-------------------------------------------------")

	// 执行OnAfterInit
	for _, c := range a.components {
		clog.Infof("[component = %s] -> OnAfterInit().", c.Name())
		c.OnAfterInit()
	}

	// 网络模块
	if a.isFrontend {
		if a.netParser == nil {
			clog.Panic("net packet parser is nil.")
		}
		a.netParser.Load(a)
	}

	clog.Info("-------------------------------------------------")
	clog.Infof("[spend time = %dms] application is running.", a.startTime.NowDiffMillisecond())
	clog.Info("-------------------------------------------------")

	// 标记为running
	atomic.AddInt32(&a.running, 1)

	//接收系统信号的chan
	sg := make(chan os.Signal, 1)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	select {
	case <-a.dieChan:
		clog.Info("invoke shutdown().")
	case s := <-sg:
		clog.Infof("receive shutdown signal = %v.", s)
	}

	// stop 状态
	atomic.StoreInt32(&a.running, 0)

	clog.Info("------- application will shutdown -------")

	if a.onShutdownFn != nil {
		for _, f := range a.onShutdownFn {
			cutils.Try(func() {
				f()
			}, func(errString string) {
				clog.Warnf("[onShutdownFn] error = %s", errString)
			})
		}
	}

	//所有组件反序
	for i := len(a.components) - 1; i >= 0; i-- {
		cutils.Try(func() {
			clog.Infof("[component = %s] -> OnBeforeStop().", a.components[i].Name())
			a.components[i].OnBeforeStop()
		}, func(errString string) {
			clog.Warnf("[component = %s] -> OnBeforeStop(). error = %s", a.components[i].Name(), errString)
		})
	}

	for i := len(a.components) - 1; i >= 0; i-- {
		cutils.Try(func() {
			clog.Infof("[component = %s] -> OnStop().", a.components[i].Name())
			a.components[i].OnStop()
		}, func(errString string) {
			clog.Warnf("[component = %s] -> OnStop(). error = %s", a.components[i].Name(), errString)
		})
	}

	clog.Info("------- application has been shutdown... -------")
}

func (a *Application) Shutdown() {
	a.dieChan <- true
}

func (a *Application) Serializer() cfacade.ISerializer {
	return a.serializer
}

func (a *Application) Discovery() cfacade.IDiscovery {
	return a.discovery
}

func (a *Application) Cluster() cfacade.ICluster {
	return a.cluster
}

func (a *Application) ActorSystem() cfacade.IActorSystem {
	return a.actorSystem
}

func (a *Application) StartTime() string {
	return a.startTime.ToDateTimeFormat()
}

func (a *Application) SetSerializer(serializer cfacade.ISerializer) {
	if a.Running() || serializer == nil {
		return
	}

	a.serializer = serializer
}

func (a *Application) SetDiscovery(discovery cfacade.IDiscovery) {
	if a.Running() || discovery == nil {
		return
	}

	a.discovery = discovery
}

func (a *Application) SetCluster(cluster cfacade.ICluster) {
	if a.Running() || cluster == nil {
		return
	}

	a.cluster = cluster
}

func (a *Application) SetNetParser(netParser cfacade.INetParser) {
	if a.Running() || netParser == nil {
		return
	}

	a.netParser = netParser
}
