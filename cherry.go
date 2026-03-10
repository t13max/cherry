package cherry

import (
	cfacade "github.com/cherry-game/cherry/facade"
	ccluster "github.com/cherry-game/cherry/net/cluster"
	cdiscovery "github.com/cherry-game/cherry/net/discovery"
)

type (
	AppBuilder struct {
		*Application
		components []cfacade.IComponent
	}
)

func Configure(profileFilePath, nodeID string, isFrontend bool, mode NodeMode) *AppBuilder {
	appBuilder := &AppBuilder{
		Application: NewApp(profileFilePath, nodeID, isFrontend, mode),
		components:  make([]cfacade.IComponent, 0),
	}

	return appBuilder
}

func ConfigureNode(node cfacade.INode, isFrontend bool, mode NodeMode) *AppBuilder {
	appBuilder := &AppBuilder{
		Application: NewAppNode(node, isFrontend, mode),
		components:  make([]cfacade.IComponent, 0),
	}

	return appBuilder
}

// Startup 启动
func (p *AppBuilder) Startup() {
	app := p.Application

	if app.NodeMode() == Cluster {
		//集群模块
		cluster := ccluster.New()
		app.SetCluster(cluster)
		app.Register(cluster)

		//服务注册发现模块
		discovery := cdiscovery.New()
		app.SetDiscovery(discovery)
		app.Register(discovery)
	}

	// Register custom components
	app.Register(p.components...)

	// startup
	app.Startup()
}

func (p *AppBuilder) Register(component ...cfacade.IComponent) {
	p.components = append(p.components, component...)
}

func (p *AppBuilder) AddActors(actors ...cfacade.IActorHandler) {
	p.actorSystem.Add(actors...)
}

func (p *AppBuilder) NetParser() cfacade.INetParser {
	return p.netParser
}

func (p *AppBuilder) SetNetParser(parser cfacade.INetParser) {
	p.netParser = parser
}
