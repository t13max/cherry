// 优先队列 最小堆实现

package cherryTimeWheel

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

// 队列中的元素
type item struct {
	Value    interface{} // 存储的对象
	Priority int64       // 优先级, 这里表示过期时间（越小越早执行）
	Index    int         // 在堆中的索引位置
}

// 优先队列, 本质是最小堆
// 下标 0 永远是 Priority 最小的元素
type priorityQueue []*item

// 创建优先队列
func newPriorityQueue(capacity int) priorityQueue {
	return make(priorityQueue, 0, capacity)
}

// Len 返回队列长度
func (pq priorityQueue) Len() int {
	return len(pq)
}

// Less 堆排序规则：Priority 小的优先
func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

// Swap 交换两个元素的位置
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

// Push 向堆中插入元素
func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	c := cap(*pq)

	// 如果容量不够则扩容为原来的2倍
	if n+1 > c {
		npq := make(priorityQueue, n, c*2)
		copy(npq, *pq)
		*pq = npq
	}

	// 扩展 slice
	*pq = (*pq)[0 : n+1]

	value := x.(*item)
	value.Index = n

	// 放到最后一个位置
	(*pq)[n] = value
}

// Pop 从堆中弹出元素
func (pq *priorityQueue) Pop() interface{} {
	n := len(*pq)
	c := cap(*pq)

	// 如果使用率太低则缩容
	if n < (c/2) && c > 25 {
		npq := make(priorityQueue, n, c/2)
		copy(npq, *pq)
		*pq = npq
	}

	value := (*pq)[n-1]
	value.Index = -1

	// slice 去掉最后一个元素
	*pq = (*pq)[0 : n-1]

	return value
}

// PeekAndShift 查看堆顶元素是否到期
// maxValue：当前时间
// 返回：
// 1. 到期的 item
// 2. 如果未到期, 返回还需要等待的时间
func (pq *priorityQueue) PeekAndShift(maxValue int64) (*item, int64) {
	// 队列为空
	if pq.Len() == 0 {
		return nil, 0
	}

	// 堆顶元素（最早过期）
	value := (*pq)[0]

	// 如果还没到过期时间
	if value.Priority > maxValue {
		return nil, value.Priority - maxValue
	}

	// 已经到期, 从堆中移除
	heap.Remove(pq, 0)

	return value, 0
}

// DelayQueue 是一个无限容量的延迟队列
// 只有当元素过期时才能被取出
// 队头始终是最早过期的元素
type DelayQueue struct {
	C        chan interface{} // 到期chan
	mu       sync.Mutex       // 保护优先队列
	pq       priorityQueue    // 最小堆
	sleeping int32            // 是否处于休眠状态（类似 runtime.timers）
	wakeupC  chan struct{}    // 用于唤醒 Poll
}

// NewDelayQueue 创建 DelayQueue
func NewDelayQueue(size int) *DelayQueue {
	return &DelayQueue{
		C:       make(chan interface{}),
		pq:      newPriorityQueue(size),
		wakeupC: make(chan struct{}),
	}
}

// Offer 向延迟队列中插入元素
// elem：元素
// expiration：过期时间（毫秒）
func (dq *DelayQueue) Offer(elem interface{}, expiration int64) {
	value := &item{
		Value:    elem,
		Priority: expiration,
	}

	dq.mu.Lock()

	// 插入最小堆
	heap.Push(&dq.pq, value)

	// 获取该元素在堆中的位置
	index := value.Index

	dq.mu.Unlock()

	// 如果是新的最早过期任务
	if index == 0 {

		// 如果当前 Poll 线程处于休眠状态, 则唤醒
		if atomic.CompareAndSwapInt32(&dq.sleeping, 1, 0) {
			dq.wakeupC <- struct{}{}
		}
	}
}

// Poll 是延迟队列的核心循环
// 持续等待任务到期, 然后把任务发送到 C 通道
func (dq *DelayQueue) Poll(exitC chan struct{}, nowF func() int64) {

	for {
		// 获取当前时间
		now := nowF()

		dq.mu.Lock()

		// 查看最早到期任务
		value, delta := dq.pq.PeekAndShift(now)

		if value == nil {
			// 队列为空 或者 任务还没到期

			// 设置为休眠状态
			// 这里必须和 PeekAndShift 保持原子性
			// 防止 Offer 与 Poll 之间发生竞态
			atomic.StoreInt32(&dq.sleeping, 1)
		}

		dq.mu.Unlock()

		if value == nil {

			// 队列为空
			if delta == 0 {
				select {

				// 等待新任务加入
				case <-dq.wakeupC:
					continue

				// 收到退出信号
				case <-exitC:
					goto exit
				}

			} else if delta > 0 {

				// 有任务, 但还没到期
				select {

				// 有更早任务加入
				case <-dq.wakeupC:
					continue

				// 等待当前任务到期
				case <-time.After(time.Duration(delta) * time.Millisecond):

					// 任务到期, 取消休眠状态
					if atomic.SwapInt32(&dq.sleeping, 0) == 0 {

						// 如果 Offer 正阻塞在 wakeupC 发送
						// 这里读取一次防止阻塞
						<-dq.wakeupC
					}

					continue

				case <-exitC:
					goto exit
				}
			}
		}

		// 任务已经到期
		select {

		// 发送到输出通道
		case dq.C <- value.Value:

		case <-exitC:
			goto exit
		}
	}

exit:

	// 重置状态
	atomic.StoreInt32(&dq.sleeping, 0)
}
