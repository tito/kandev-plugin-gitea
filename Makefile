.PHONY: build run test test-backend test-recipes typecheck-recipes audit-recipes \
	fmt vet package package-host verify-package verify-package-host clean

BIN := bin/kandev-plugin-gitea
VERSION := 0.1.3
STAGE := .build/stage
PKG_OUT := kandev-plugin-gitea-$(VERSION).tar.gz

KANDEV_SDK := ../kandev/apps/backend

build:
	mkdir -p bin
	go build -o $(BIN) ./server/...

run: build
	./$(BIN)

test: test-backend typecheck-recipes test-recipes

test-backend:
	go test ./server/... ./recipes/source-control/server/...

test-recipes:
	npm run test:recipes

typecheck-recipes:
	npm run typecheck:recipes

audit-recipes:
	npm audit --audit-level=high

fmt:
	gofmt -l .

vet:
	go vet ./server/... ./recipes/source-control/server/...

package:
	rm -rf $(STAGE)
	mkdir -p $(STAGE)/server
	cp manifest.yaml $(STAGE)/manifest.yaml
	cp -r ui $(STAGE)/ui
	GOOS=linux   GOARCH=amd64 go build -o $(STAGE)/server/plugin-linux-amd64       ./server
	GOOS=linux   GOARCH=arm64 go build -o $(STAGE)/server/plugin-linux-arm64       ./server
	GOOS=darwin  GOARCH=amd64 go build -o $(STAGE)/server/plugin-darwin-amd64      ./server
	GOOS=darwin  GOARCH=arm64 go build -o $(STAGE)/server/plugin-darwin-arm64      ./server
	GOOS=windows GOARCH=amd64 go build -o $(STAGE)/server/plugin-windows-amd64.exe ./server
	cd $(KANDEV_SDK) && go run ./cmd/plugin-pack -dir $(CURDIR)/$(STAGE) -out $(CURDIR)/$(PKG_OUT)
	rm -rf $(STAGE)
	@echo "Wrote $(PKG_OUT)"

package-host:
	rm -rf $(STAGE)
	mkdir -p $(STAGE)/server
	cp manifest.yaml $(STAGE)/manifest.yaml
	cp -r ui $(STAGE)/ui
	go build -o $(STAGE)/server/plugin-$$(go env GOOS)-$$(go env GOARCH)$$(go env GOEXE) ./server
	cd $(KANDEV_SDK) && go run ./cmd/plugin-pack -dir $(CURDIR)/$(STAGE) -out $(CURDIR)/$(PKG_OUT) -platform-only
	rm -rf $(STAGE)
	@echo "Wrote $(PKG_OUT)"

verify-package: package
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		tar -xzf "$(PKG_OUT)" -C "$$tmp"; \
		test -f "$$tmp/manifest.yaml"; \
		test -f "$$tmp/ui/bundle.js"; \
		test -f "$$tmp/checksums.txt"; \
		for executable in \
			plugin-linux-amd64 plugin-linux-arm64 \
			plugin-darwin-amd64 plugin-darwin-arm64 \
			plugin-windows-amd64.exe; do \
			test -f "$$tmp/server/$$executable"; \
		done; \
		test ! -e "$$tmp/recipes"; \
		test ! -e "$$tmp/package.json"; \
		if command -v sha256sum >/dev/null 2>&1; then \
			(cd "$$tmp" && sha256sum -c checksums.txt); \
		else \
			(cd "$$tmp" && shasum -a 256 -c checksums.txt); \
		fi

verify-package-host: package-host
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		tar -xzf "$(PKG_OUT)" -C "$$tmp"; \
		host_executable="plugin-$$(go env GOOS)-$$(go env GOARCH)$$(go env GOEXE)"; \
		test -f "$$tmp/manifest.yaml"; \
		test -f "$$tmp/ui/bundle.js"; \
		test -f "$$tmp/checksums.txt"; \
		test -f "$$tmp/server/$$host_executable"; \
		test ! -e "$$tmp/recipes"; \
		test ! -e "$$tmp/package.json"; \
		if command -v sha256sum >/dev/null 2>&1; then \
			(cd "$$tmp" && sha256sum -c checksums.txt); \
		else \
			(cd "$$tmp" && shasum -a 256 -c checksums.txt); \
		fi

clean:
	rm -rf bin $(STAGE) kandev-plugin-gitea-*.tar.gz
