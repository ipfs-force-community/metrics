### Venus集群消息全链路追踪部署配置及接入代码说明

### Jeager服务环境搭建

请参考链接:
[jaeger-elasticsearch-deploy.](https://github.com/zl03jsj/jaeger-elasticsearch-deploy.git)

### venus服务

在~/.venus/config.json中修改tracing段:
```json
"tracing": {
		"jaegerTracingEnabled": true,
		"probabilitySampler": 1,
		"jaegerEndpoint": "192.168.1.125:6831",
		"servername": "venus-node-mac-zl"
	}
```
- jagerTracingEnabled, 是否启用链路追踪功能
- probabilitySampler:链路tracing的采样率, 小数类型, 0-1
- jaegerEndpoint: jaeger服务的proxy, 使用udp协议, 格式为host:port
- servername: 注册服务节点的名称

### venus-gateway服务
由于venus-gateway没有相关的配置文件, 所以提供了2种方式添加:
#### 方式一, 通过命令行参数设置
```shell
./venus-gateway run --jaeger-proxy=192.168.1.125:6831 --trace_sampler=1.0 --auth-url=192.168.1.125:8989
```
#### 方式二, 通过环境变量设置
```shell
export VENUS_GATEWAY_JAEGER_PROXY=192.168.1.125:6831
export VENUS_GATEWAY_TRACE_SAMPLER=1.0
./venus-gateway run --auth-url=192.168.1.125:8989
```
如果配置了`jaeger-proxy`或者`VENUS_GATEWAY_JAEGER_PROXY`则表示启动链路追踪功能, 
`trace_sampler`为可选参数, 默认值为1.0, 
`trace_sampler`为可选参数, 默认值为:venus-gateway.

### Venus-auth

在其配置文件中, 添加Trace段,配置如下:

```yaml
[Trace] 
  JaegerTracingEnabled = true 
  ProbabilitySampler = 1.0 
  JaegerEndpoint = "192.168.1.125:6831" 
  ServerName = "venus-auth"
```

### 其它服务接入Jaeger示例

1. 在[ifs-force-community/metircs](https://github.com/ipfs-force-community/metrics.git)已经封装好了相关代码, 使用时只需要引入这个包:

	```go
	import "github.com/ipfs-force-community/metrics"
	```

2. 接入jaeger配置需要提供相关的配置项:

   ```go
   type TraceConfig struct {
   	JaegerTracingEnabled bool    `json:"jaegerTracingEnabled"`
   	ProbabilitySampler   float64 `json:"probabilitySampler"`
   	JaegerEndpoint       string  `json:"jaegerEndpoint"`
   	ServerName           string  `json:"servername"`
   }
   ```
3. 注册jaeger-exporter

	```go
	tCnf := cctx.Context.Value("trace-config").(*metrics.TraceConfig)
	if repoter, err := metrics.RegisterJaeger(tCnf.ServerName, tCnf); err != nil {
		log.Fatalf("register %s JaegerRepoter to %s failed:%s",
			tCnf.ServerName, tCnf.JaegerEndpoint)
	} else if repoter != nil {
		defer repoter.Flush()
	}
	```
	
	注册成功够, 在**<font color=red>服务退出</font>**时, 需要调用`repoter.Flush()`.

#### 使用filcoin官方[go-jsonrpc](https://github.com/filecoin-project/go-jsonrpc.git)作为服务间通讯

使用go-fsonrpc作为服务间通讯组件, 不需要做任何修改, 所有的trace都集成在go-jsonrpc内部, 会自动上报.

#### 使用http-rpc作为服务间通讯

参考[jaeger-example]()

#### 服务器端
```go
package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/ipfs-force-community/metrics"
	"go.opencensus.io/plugin/ochttp"
	"net/http"
	"time"
)

func main() {
	tCnf := &metrics.TraceConfig{
		JaegerTracingEnabled: true,
		ProbabilitySampler:   1.0,
		JaegerEndpoint:       "192.168.1.125:6831",
		ServerName:           "server-test",
	}

	repoter, err := metrics.RegisterJaeger(tCnf.ServerName, tCnf)
	if err != nil {
		panic(fmt.Sprintf("register %s JaegerRepoter to %s failed:%s",
			tCnf.ServerName, tCnf.JaegerEndpoint))
	}
	defer repoter.Flush()
	router := mux.NewRouter()
	
	router.HandleFunc("/rpc/v0/ping", func(w http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 128)
	
		n, _ := req.Body.Read(buf)
	
		fmt.Printf("request body content:%s\n", string(buf[:n]))
	
		_, _ = w.Write([]byte("pong"))
	})
	
	server := &http.Server{
		Addr:           ":8080",
		Handler:        &ochttp.Handler{Handler: router},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
```

#### 客户端
```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ipfs-force-community/metrics"
	"go.opencensus.io/plugin/ochttp"
	"net/http"
	"time"
)
func main() {
	tCnf := &metrics.TraceConfig{
		JaegerTracingEnabled: true,
		ProbabilitySampler:   1.0,
		JaegerEndpoint:       "192.168.1.125:6831",
		ServerName:           "client-test",
	}

	repoter, err := metrics.RegisterJaeger(tCnf.ServerName, tCnf)
	if err != nil {
		panic(fmt.Sprintf("register %s JaegerRepoter to %s failed:%s",
			tCnf.ServerName, tCnf.JaegerEndpoint))
	}
	defer repoter.Flush()

	// In other usages, the context would have been passed down after starting some traces.
	ctx := context.Background()
	req, _ := http.NewRequest("GET",
		"http://localhost:8080/rpc/v0/ping",
		bytes.NewBuffer([]byte("ping")))

	// It is imperative that req.WithContext is used to
	// propagate context and use it in the request.
	req = req.WithContext(ctx)

	client := &http.Client{Transport: &ochttp.Transport{}}

	var buf = make([]byte, 128)

	for {
		res, err := client.Do(req)
		if err != nil {
			panic(fmt.Errorf("Failed to make the request: %s", err.Error()))
		}
		n, _ := res.Body.Read(buf)
		fmt.Printf("%s\n", string(buf[:n]))
		time.Sleep(time.Second * 3)
	}
}
```

运行后查看jaeger-ui, 可以看到效果:
<img src="https://raw.githubusercontent.com/ipfs-force-community/metrics/master/example/demo.png" style="zoom: 50%;" />
