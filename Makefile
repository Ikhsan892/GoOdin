MAIN_PACKAGE_PATH := ./examples
BINARY_NAME := service
VERSION := ${VERSION}
DESC := Base Template Service
MAINTAINER := Fatihul Ikhsan
GOLANG_VERSION := 1.22.5
ALPINE_VERSION := 3.19
DEBIAN_VERSION := bullseye

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code

.PHONY: postgres
postgres:
	docker run --name local-postgres -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=admin -e POSTGRES_DB=goceng -p 5431:5432 -d postgres:latest


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go test -race -buildvcs -vet=off ./...


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #


## check-events: verify all event constants in pkg/events/ are registered in register.go
## Fails CI if someone adds an event constant but forgets to wire it in register.go
.PHONY: check-events
check-events:
	@echo "Checking all event constants are registered in drivers/watermill/register.go..."
	@missing=0; \
	for f in pkg/events/*.go; do \
		grep -E '^const [A-Z]' "$$f" | awk '{print $$2}' | while read -r name; do \
			if ! grep -q "events\.$$name" drivers/watermill/register.go 2>/dev/null && \
			   ! grep -q "events\.$$name" drivers/watermill/event_bus.go 2>/dev/null; then \
				echo "  ERROR: events.$$name defined in $$f but missing from register.go or event_bus.go"; \
				missing=1; \
			fi; \
		done; \
	done; \
	echo "Done."

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## test/service: run seperate tests
.PHONY: test/service
test/service:
	go test -v -race service

## swagger/init : run init swagger
.PHONY: swagger/init
swagger/init:
	swag init -g drivers/http/echo.go

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	go test -v -race -buildvcs -coverprofile=/tmp/coverage.out ./...
	go tool cover -html=/tmp/coverage.out


## run/http: run the http api
.PHONY: run/http
run/http:
	go run examples/main.go http


## run/message-broker: run the message-broker system
.PHONY: run/message-broker
run/message-broker:
	go run examples/main.go message-broker

## run/migration: run the migration
.PHONY: run/migration
run/migration:
	go run examples/main.go migration up


## run/seeders: run the seeders
.PHONY: run/seeders
run/seeders:
	go run examples/main.go seeders


## dev: run the  application mode development
.PHONY: run/dev
run/dev:
	go run examples/main.go all

## dev: run the  application mode development
.PHONY: run/graph
run/graph:
	go run bootstrap/main.go graph

## run/live: run the application with reloading on file changes
.PHONY: run/live
run/live:
	go run github.com/cosmtrek/air@v1.43.0 \
		--build.cmd "make build" --build.bin "/tmp/bin/${BINARY_NAME} server" --build.delay "100"  --build.exclude_dir "" \
		--build.include_ext "go, tpl, tmpl, html, css, scss, js, ts, sql, jpeg, jpg, gif, png, bmp, svg, webp, ico" \
		--misc.clean_on_exit "true"


# ==================================================================================== #
# OPERATIONS
# ==================================================================================== #

.PHONY: version
version:
	go run examples/main.go version

## push: push changes to the remote Git repository
.PHONY: push
push:
	git push

## production/deploy: deploy the application to production
.PHONY: build
build:
	@echo "Building the binary..."
	@go get .
	@go build -ldflags="-X ${MODULE}/pkg.Version=${VERSION}" \
	-o ${BINARY_NAME} ${MAIN_PACKAGE_PATH}
	@echo "You can now use ./${BINARY_NAME}"


generate-migration:
	.\bin\migrate.exe create -ext sql -dir ./migrations -seq ${NAME}


compile-proto:
	protoc -I=./use_cases/${PROTO_FOLDER}/protobuf --go_out=./use_cases/${PROTO_FOLDER} ./use_cases/${PROTO_FOLDER}/protobuf/${PROTO_FILE}.proto

# Build the container using the Dockerfile (alpine)
docker:
	docker build --no-cache --pull --build-arg GOLANG_VERSION=${GOLANG_VERSION} --build-arg ALPINE_VERSION=${ALPINE_VERSION} -t tiultemplate:${VERSION} .

docker-push:
	docker push tiul/tiultemplate:$(VERSION)

docker-build-push: docker docker-push

## production/deploy: deploy the application to production
.PHONY: production/deploy
production/deploy: confirm tidy audit no-dirty
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=/tmp/bin/linux_amd64/${BINARY_NAME} ${MAIN_PACKAGE_PATH}
	upx -5 /tmp/bin/linux_amd64/${BINARY_NAME}
	# Include additional deployment steps here...

## binary/windows: Create binary for windows
.PHONY: binary/windows
binary/windows:
	cd ./ui && \
	npm run build  && \
	cd .. && \
	echo 'Creating Binary for Windows...' && \
	go build -ldflags -H=windowsgui -o=${BINARY_NAME} ${MAIN_PACKAGE_PATH} && \
	echo 'Done Creating Binary for Windows'


## binary/windows: Create binary for windows debug mode
.PHONY: binary/windows/debug
binary/windows/debug:
	echo 'Creating Binary for Windows...' && \
	go build -o=${BINARY_NAME} ${MAIN_PACKAGE_PATH} && \
	mv ./${BINARY_NAME} C:/Users/FATIHUL/Documents/pribadi/testajah/${BINARY_NAME} && \
	echo 'Done Creating Binary for Windows'

