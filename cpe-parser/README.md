# CPE Parser Module
- [x] Implement app-specific parser
  `func ParseValue(body []byte) (map[string][]float64, error)`
- [x] Implement common work to [read log from cos](common/cos.go) and [push to Prometheus's PushGateway](common/pusher.go)
    - create three stat gauge per a key from values: [key]_min_val, [key]_max_val, [key]_avg_val
    - group by benchmark name
    - push to PushGatewayURL
- [x] Implement REST API to receive request to read and push log metric
- [x] Docker Image and Kubernetes YAML with Secret Environments

## Build and Push image
```bash
export IMAGE_REGISTRY=[your image regirstry]
export VERSION=[your image tag]
chmod +x build_push.sh
./build_push.sh
```

## Set environments and Deploy
Pre-deploy Requirements:
- deploy secret key for image pull: $PULL_SECRET  [[read more](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)]
- deploy secret key for S3 cloud object storage: $COS_SECRET
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: cpe-cos-key
  type: Opaque
  stringData:
    rawBucketName: [bucketName]
    apiKey: [apiKey]
    serviceInstanceID: [instance ID]
    authEndpoint: [authentication endpoint]
    serviceEndpoint: [service endpoint]
  ```
- deploy prometheus push gateway [[read more](https://github.com/prometheus/pushgateway)]

Deployment Steps:
1. set all environments
  ```bash
  export PARSER_NAMESPACE=[parser namespace]
  export IMAGE_REGISTRY=[your image regirstry] # if not set yet
  export VERSION=[your image tag] # if not set yet
  export PUSHGATEWAY_URL=[pushgateway service].[service namespace]:[service port]
  export PULL_SECRET=[image pull secret name]
  export COS_SECRET=[storage secret name]
  ```
2. replace the template
  ```bash
  envsubst < deploy_template.yaml > deploy.yaml
  ```
3. deploy
  ```bash
  kubectl create -f deploy.yaml
  ```

Service Point: `http://cpe-parser.cpe-operator-system:80`

## Add new parser

1. Create new parser struct with BaseParser abstraction and put in parser folder (see [example](parser/default.go))
```go
package parser

type CustomParser struct {
	*BaseParser
}

func NewCustomParser() *CustomParser {
	newParser := &CustomParser{}
	abs := &BaseParser{
		Parser: newParser,
	}
	newParser.BaseParser = abs
	return newParser
}

func (p *CustomParser) ParseValue(body []byte) (map[string]interface{}, error) {
	values := make(map[string]interface{})
  // values can be float64, []float64, []parser.ValueWithLabels, or []parser.ValuesWithLabels
	// Put your parsing code here where body is log bytes
	return values, nil
}

func (p *CustomParser) GetPerformanceValue(values map[string]interface{}) (string, float64) {

  // Put your performance key,value extraction here
  // values = return values from ParseValue function

  return performanceKey, performanceValue
}
```
2. add your parser to following code in api.go
```go
// add parser key -> module map here
var codaitParser parser.Parser = parser.NewCodaitParser()
var defaultParser parser.Parser = parser.NewDefaultParser()
var newParser parser.Parser = parser.NewCustomParser()

var parserMap map[string]parser.Parser = map[string]parser.Parser{
	"codait":  codaitParser,
	"default": defaultParser,
  "customKey": customParser,
}
```
3. rebuild and push
```bash
./build_push.sh
```
