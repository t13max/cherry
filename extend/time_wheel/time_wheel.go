/**
 * 时间轮实现
 * 基于分层时间轮(Hierarchical Timing Wheels)实现定时任务调度
 * 支持一次性任务、周期任务，并通过DelayQueue驱动时间推进
 *
 * Author: t13max
 */

// Package cherryTimeWheel file from https://github.com/RussellLuo/timingwheel
package cherryTimeWheel

import (
	"sync/atomic"
	"time"
	"unsafe"

	cutils "github.com/cherry-game/cherry/extend/utils"
	clog "github.com/cherry-game/cherry/logger"
)

// TimeWheel 是分层时间轮的核心结构
type TimeWheel struct {
	tick          int64            // 时间轮最小刻度(ms)，每个slot代表的时间
	wheelSize     int64            // 时间轮槽数量
	interval      int64            // 整个时间轮一圈代表的时间 = tick * wheelSize
	currentTime   int64            // 当前时间(毫秒)
	buckets       []*bucket        // 每个槽对应一个bucket
	queue         *DelayQueue      // 延迟队列，用于按时间驱动bucket触发
	overflowWheel unsafe.Pointer   // 上层时间轮(用于更长时间任务)
	exitC         chan struct{}    // 停止信号
	waitGroup     waitGroupWrapper // goroutine等待组
}

// NewTimeWheel 创建时间轮
func NewTimeWheel(tick time.Duration, wheelSize int64) *TimeWheel {
	// 转成毫秒
	tickMs := int64(tick / time.Millisecond)
	if tickMs <= 0 {
		clog.Error("tick must be greater than or equal to 1ms")
		return nil
	}

	// 当前时间(ms)
	startMs := TimeToMS(time.Now().UTC())

	return newTimingWheel(
		tickMs,
		wheelSize,
		startMs,
		NewDelayQueue(int(wheelSize)), // 创建延迟队列
	)
}

// newTimingWheel 实际创建时间轮
func newTimingWheel(tickMs int64, wheelSize int64, startMs int64, queue *DelayQueue) *TimeWheel {
	// 创建bucket数组
	buckets := make([]*bucket, wheelSize)
	for i := range buckets {
		buckets[i] = newBucket() // 每个slot一个bucket
	}

	return &TimeWheel{
		tick:        tickMs,                    // 每个slot时间
		wheelSize:   wheelSize,                 // slot数量
		currentTime: truncate(startMs, tickMs), // 当前时间(对齐tick)
		interval:    tickMs * wheelSize,        // 一圈时间
		buckets:     buckets,                   // bucket列表
		queue:       queue,                     // delay queue
		exitC:       make(chan struct{}),       // 退出信号
	}
}

// add 将timer加入当前时间轮
func (tw *TimeWheel) add(t *Timer) bool {

	// 读取当前时间
	currentTime := atomic.LoadInt64(&tw.currentTime)

	// 如果任务已经过期
	if t.expiration < currentTime+tw.tick {
		// Already expired
		return false
	}

	// 如果任务在当前时间轮范围内
	if t.expiration < currentTime+tw.interval {

		// 计算虚拟slot位置
		virtualID := t.expiration / tw.tick

		// 找到bucket
		b := tw.buckets[virtualID%tw.wheelSize]

		// 放入bucket
		b.Add(t)

		// 设置bucket过期时间
		if b.SetExpiration(virtualID * tw.tick) {

			// bucket第一次被使用时，需要放入delay queue
			tw.queue.Offer(b, b.Expiration())
		}
		return true
	} else {

		// 超出当前时间轮范围，需要放到更高层时间轮

		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)

		// 如果还没有创建上层时间轮
		if overflowWheel == nil {

			atomic.CompareAndSwapPointer(
				&tw.overflowWheel,
				nil,
				unsafe.Pointer(newTimingWheel(
					tw.interval,  // tick变大
					tw.wheelSize, // 槽数量一样
					currentTime,
					tw.queue, // 共用delay queue
				)),
			)

			overflowWheel = atomic.LoadPointer(&tw.overflowWheel)
		}

		// 递归加入上层时间轮
		return (*TimeWheel)(overflowWheel).add(t)
	}
}

// addOrRun 添加timer，如果已过期则直接执行
func (tw *TimeWheel) addOrRun(t *Timer) {

	// 尝试加入时间轮
	if !tw.add(t) {

		// 已过期，直接执行

		if t.isAsync {
			// 异步执行
			go t.task()
		} else {
			// 同步执行
			t.task()
		}
	}
}

// 推进时间轮时钟
func (tw *TimeWheel) advanceClock(expiration int64) {

	currentTime := atomic.LoadInt64(&tw.currentTime)

	// 如果bucket过期时间超过当前tick
	if expiration >= currentTime+tw.tick {

		// 更新时间并对齐tick
		currentTime = truncate(expiration, tw.tick)

		atomic.StoreInt64(&tw.currentTime, currentTime)

		// 如果存在上层时间轮，也推进
		overflowWheel := atomic.LoadPointer(&tw.overflowWheel)
		if overflowWheel != nil {
			(*TimeWheel)(overflowWheel).advanceClock(currentTime)
		}
	}
}

// Start 启动时间轮
func (tw *TimeWheel) Start() {

	// goroutine1: 轮询DelayQueue
	tw.waitGroup.Wrap(func() {

		tw.queue.Poll(tw.exitC, func() int64 {
			// 返回当前时间(ms)
			return TimeToMS(time.Now().UTC())
		})
	})

	// goroutine2: 处理过期bucket
	tw.waitGroup.Wrap(func() {

		for {
			select {

			// 有bucket到期
			case elem := <-tw.queue.C:

				b := elem.(*bucket)

				// 推进时间
				tw.advanceClock(b.Expiration())

				// 执行bucket中的任务
				b.Flush(tw.addOrRun)

			case <-tw.exitC:
				return
			}
		}
	})
}

// Stop 停止时间轮
func (tw *TimeWheel) Stop() {

	// 关闭退出信号
	close(tw.exitC)

	// 等待goroutine退出
	tw.waitGroup.Wait()
}

// AfterFunc 延迟执行任务
func (tw *TimeWheel) AfterFunc(id uint64, d time.Duration, f func(), async ...bool) *Timer {

	t := &Timer{
		id:         id,                                // 任务ID
		expiration: TimeToMS(time.Now().UTC().Add(d)), // 到期时间
		task:       f,                                 // 任务函数
		isAsync:    getAsyncValue(async...),           // 是否异步执行
	}

	// 加入时间轮
	tw.addOrRun(t)

	return t
}

// AddEveryFunc 周期执行任务
func (tw *TimeWheel) AddEveryFunc(id uint64, d time.Duration, f func(), async ...bool) *Timer {

	return tw.ScheduleFunc(id, &EverySchedule{Interval: d}, f, async...)
}

// BuildAfterFunc 创建延迟任务(自动生成ID)
func (tw *TimeWheel) BuildAfterFunc(d time.Duration, f func()) *Timer {

	id := NextID()

	return tw.AfterFunc(id, d, f)
}

// BuildEveryFunc 创建周期任务(自动生成ID)
func (tw *TimeWheel) BuildEveryFunc(d time.Duration, f func(), async ...bool) *Timer {

	id := NextID()

	return tw.AddEveryFunc(id, d, f, async...)
}

// ScheduleFunc 按调度器执行任务
func (tw *TimeWheel) ScheduleFunc(id uint64, s Scheduler, f func(), async ...bool) *Timer {

	// 获取第一次执行时间
	expiration := s.Next(time.Now())

	if expiration.IsZero() {
		// 没有调度时间
		return nil
	}

	t := &Timer{
		id:         id,
		expiration: TimeToMS(expiration),
		isAsync:    getAsyncValue(async...),
	}

	// 包装任务函数
	t.task = func() {

		// 获取下一次执行时间
		nextExpiration := s.Next(MSToTime(t.expiration))

		if !expiration.IsZero() {

			// 更新过期时间
			t.expiration = TimeToMS(nextExpiration)

			// 重新加入时间轮
			tw.addOrRun(t)
		}

		// 执行任务并捕获panic
		cutils.Try(f, func(errString string) {
			clog.Warn(errString)
		})
	}

	tw.addOrRun(t)

	return t
}

// NextID 获取下一个任务ID
func (tw *TimeWheel) NextID() uint64 {
	return NextID()
}

// 获取async参数
func getAsyncValue(asyncTask ...bool) bool {

	if len(asyncTask) > 0 {
		return asyncTask[0]
	}

	return false
}
