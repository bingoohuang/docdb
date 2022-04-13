module github.com/eatonphil/docdb

go 1.18

replace go.etcd.io/bbolt => github.com/flower-corp/bbolt v1.3.7-0.20220315040627-32fed02add8f

require (
	github.com/akrylysov/pogreb v0.10.1
	github.com/bingoohuang/gg v0.0.0-20220401032752-6d4a79888f0e
	github.com/cockroachdb/pebble v0.0.0-20220331220335-53fd136b2293
	github.com/flower-corp/lotusdb v1.0.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/stretchr/testify v1.7.1
)

require (
	go.etcd.io/bbolt v1.3.6 // indirect
	go.uber.org/atomic v1.7.0 // indirect
)

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/cockroachdb/errors v1.8.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20190617123548-eb05cc24525f // indirect
	github.com/cockroachdb/redact v1.0.8 // indirect
	github.com/cockroachdb/sentry-go v0.6.1-cockroachdb.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/klauspost/compress v1.11.7 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.8.0
	golang.org/x/exp v0.0.0-20200513190911-00229845015e // indirect
	golang.org/x/sys v0.0.0-20211210111614-af8b64212486 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
