module github.com/Hirom0112/Base-GridOS/tests/contract

go 1.24.0

require (
	github.com/Hirom0112/Base-GridOS/contracts/gen/go v0.0.0
	google.golang.org/protobuf v1.36.10
)

replace github.com/Hirom0112/Base-GridOS/contracts/gen/go => ../../contracts/gen/go
