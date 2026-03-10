//邮箱

package cherryActor

import (
	creflect "github.com/cherry-game/cherry/extend/reflect"
	ctime "github.com/cherry-game/cherry/extend/time"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

type mailbox struct {
	queue                                 // queue
	name    string                        // 邮箱名
	funcMap map[string]*creflect.FuncInfo // 已注册的函数
}

func newMailbox(name string) mailbox {
	return mailbox{
		queue:   newQueue(),
		name:    name,
		funcMap: make(map[string]*creflect.FuncInfo),
	}
}

// Register 注册函数
func (p *mailbox) Register(funcName string, fn interface{}) {
	if funcName == "" || len(funcName) < 1 {
		clog.Errorf("[%s] Func name is empty.", fn)
		return
	}

	funcInfo, err := creflect.GetFuncInfo(fn)
	if err != nil {
		clog.Errorf("funcName = %s, err = %v", funcName, err)
		return
	}
	//判重
	if _, found := p.funcMap[funcName]; found {
		clog.Errorf("funcName = %s, already exists.", funcName)
		return
	}
	//添加
	p.funcMap[funcName] = &funcInfo
}

// GetFuncInfo 根据函数名获取函数信息
func (p *mailbox) GetFuncInfo(funcName string) (*creflect.FuncInfo, bool) {
	funcInfo, found := p.funcMap[funcName]
	return funcInfo, found
}

// Pop 出队一个Message
func (p *mailbox) Pop() *cfacade.Message {
	v := p.queue.Pop()
	if v == nil {
		return nil
	}

	msg, ok := v.(*cfacade.Message)
	if !ok {
		clog.Warnf("Convert to *Message fail. v = %+v", v)
		return nil
	}

	return msg
}

// Push 入队一个Message
func (p *mailbox) Push(m *cfacade.Message) {
	if m != nil {
		m.PostTime = ctime.Now().ToMillisecond()
		p.queue.Push(m)
	}
}

// stop 被Actor调用
func (p *mailbox) onStop() {
	for key := range p.funcMap {
		delete(p.funcMap, key)
	}

	p.queue.Destroy()
}
