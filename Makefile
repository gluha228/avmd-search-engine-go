build:
	go build -o api_server ./cmd/apiserver/api_server.go

run:
	env $$(cat .env | xargs) go run ./cmd/apiserver/api_server.go

oapi-gen:
	cd api/gen && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -config cfg.yaml ../openapi.yaml

test:
	go test ./...
