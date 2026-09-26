module github.com/Hirom0112/Base-GridOS/tests/integration

go 1.26.0

require github.com/Hirom0112/Base-GridOS/contracts/gen/go v0.0.0

require google.golang.org/protobuf v1.36.12 // indirect

replace github.com/Hirom0112/Base-GridOS/contracts/gen/go => ../../contracts/gen/go

replace github.com/Hirom0112/Base-GridOS/services/control => ../../services/control
