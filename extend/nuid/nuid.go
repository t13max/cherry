//分布式id 基于随机数和时间戳 比UUID短

package cherryNUID

import "github.com/nats-io/nuid"

var (
	id = nuid.New()
)

func Next() string {
	return id.Next()
}
