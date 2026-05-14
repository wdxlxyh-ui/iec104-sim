PROJECT   := iec104-sim
VERSION   := 2.1.5
LDFLAGS   := -ldflags="-s -w -X main.version=$(VERSION)"
DIST_DIR  := dist
BIN_DIR   := bin

.PHONY: all build-linux-amd64 build-linux-arm64 build-windows build-all \
        deb-amd64 deb-arm64 deb compress clean smoke fmt vet \
        web-build dist

all: build-linux-amd64

# ── Linux amd64 ─────────────────────────────────────────
build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT)-linux-amd64 ./cmd/iec104-sim/

# ── Linux arm64 (aarch64) ──────────────────────────────
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT)-linux-arm64 ./cmd/iec104-sim/

# ── Windows amd64 ─────────────────────────────────────
build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT).exe ./cmd/iec104-sim/

# ── 全部二进制 ──────────────────────────────────────────
build-all: build-linux-amd64 build-linux-arm64 build-windows

# ── Web 前端构建 (需要 Node.js) ─────────────────────────
web-build:
	@if [ -d web ]; then \
		cd web && npm install --silent && npm run build; \
	else \
		echo "web/ directory not found, skipping frontend build"; \
	fi

# ── 完整构建 (含前端) ───────────────────────────────────
build-full: web-build build-linux-amd64

# ── 三平台全部打包 tar.gz/zip ──────────────────────────
dist: web-build build-all
	@echo ""
	@echo "=== 打包三平台发行包 ==="
	@echo ""
	# Linux amd64
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/bin
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/config
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/logs
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/resources
	cp $(BIN_DIR)/$(PROJECT)-linux-amd64 $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/bin/$(PROJECT)
	cp scripts/start.sh scripts/stop.sh scripts/restart.sh $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/bin/
	chmod +x $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/bin/*.sh
	echo '[]' > $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/config/instances.json
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/logs/.gitkeep
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/resources/.gitkeep
	@if [ -d web/dist ]; then \
		cp -r web/dist $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64/web; \
	fi
	cd $(DIST_DIR) && tar czf $(PROJECT)-v$(VERSION)-linux-amd64.tar.gz $(PROJECT)-v$(VERSION)-linux-amd64/
	@rm -rf $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64
	@echo "  ✔ $(PROJECT)-v$(VERSION)-linux-amd64.tar.gz"
	# Linux arm64
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/bin
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/config
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/logs
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/resources
	cp $(BIN_DIR)/$(PROJECT)-linux-arm64 $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/bin/$(PROJECT)
	cp scripts/start.sh scripts/stop.sh scripts/restart.sh $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/bin/
	chmod +x $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/bin/*.sh
	echo '[]' > $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/config/instances.json
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/logs/.gitkeep
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/resources/.gitkeep
	@if [ -d web/dist ]; then \
		cp -r web/dist $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64/web; \
	fi
	cd $(DIST_DIR) && tar czf $(PROJECT)-v$(VERSION)-linux-arm64.tar.gz $(PROJECT)-v$(VERSION)-linux-arm64/
	@rm -rf $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64
	@echo "  ✔ $(PROJECT)-v$(VERSION)-linux-arm64.tar.gz"
	# Windows amd64 (zip)
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/bin
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/scripts
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/config
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/logs
	mkdir -p $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/resources
	cp $(BIN_DIR)/$(PROJECT).exe $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/bin/$(PROJECT).exe
	cp scripts/start.bat scripts/stop.bat scripts/restart.bat $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/scripts/
	echo '[]' > $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/config/instances.json
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/logs/.gitkeep
	touch $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/resources/.gitkeep
	@if [ -d web/dist ]; then \
		cp -r web/dist $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64/web; \
	fi
	cd $(DIST_DIR) && zip -rq $(PROJECT)-v$(VERSION)-windows-amd64.zip $(PROJECT)-v$(VERSION)-windows-amd64/
	@rm -rf $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64
	@echo "  ✔ $(PROJECT)-v$(VERSION)-windows-amd64.zip"
	@echo ""
	@echo "=== 三平台打包完成 ==="
	@ls -lh $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-amd64.tar.gz
	@ls -lh $(DIST_DIR)/$(PROJECT)-v$(VERSION)-linux-arm64.tar.gz
	@ls -lh $(DIST_DIR)/$(PROJECT)-v$(VERSION)-windows-amd64.zip
	@echo ""

# ── .deb 打包 ───────────────────────────────────────────
deb-amd64: build-linux-amd64
	@mkdir -p /tmp/deb-amd64/DEBIAN /tmp/deb-amd64/usr/local/bin
	cp $(BIN_DIR)/$(PROJECT)-linux-amd64 /tmp/deb-amd64/usr/local/bin/$(PROJECT)
	chmod 755 /tmp/deb-amd64/usr/local/bin/$(PROJECT)
	printf 'Package: %s\nVersion: %s\nSection: utils\nPriority: optional\nArchitecture: amd64\nMaintainer: IEC104 Simulator <dev@example.com>\nDescription: IEC 60870-5-104 Simulator with Web Management\n Supports multi-instance IEC104 simulation with\n web-based configuration and monitoring.\nBuilt-Using: go1.22.5\n' $(PROJECT) $(VERSION) > /tmp/deb-amd64/DEBIAN/control
	cd /tmp/deb-amd64 && find . -type f ! -path './DEBIAN/*' -exec md5sum {} \; > DEBIAN/md5sums
	dpkg-deb --build /tmp/deb-amd64 $(BIN_DIR)/$(PROJECT)_$(VERSION)_amd64.deb
	@rm -rf /tmp/deb-amd64

deb-arm64: build-linux-arm64
	@mkdir -p /tmp/deb-arm64/DEBIAN /tmp/deb-arm64/usr/local/bin
	cp $(BIN_DIR)/$(PROJECT)-linux-arm64 /tmp/deb-arm64/usr/local/bin/$(PROJECT)
	chmod 755 /tmp/deb-arm64/usr/local/bin/$(PROJECT)
	printf 'Package: %s\nVersion: %s\nSection: utils\nPriority: optional\nArchitecture: arm64\nMaintainer: IEC104 Simulator <dev@example.com>\nDescription: IEC 60870-5-104 Simulator with Web Management\n Supports multi-instance IEC104 simulation with\n web-based configuration and monitoring.\nBuilt-Using: go1.22.5\n' $(PROJECT) $(VERSION) > /tmp/deb-arm64/DEBIAN/control
	cd /tmp/deb-arm64 && find . -type f ! -path './DEBIAN/*' -exec md5sum {} \; > DEBIAN/md5sums
	dpkg-deb --build /tmp/deb-arm64 $(BIN_DIR)/$(PROJECT)_$(VERSION)_arm64.deb
	@rm -rf /tmp/deb-arm64

deb: deb-amd64 deb-arm64

# ── UPX 压缩 ────────────────────────────────────────────
compress: build-linux-amd64
	upx --best $(BIN_DIR)/$(PROJECT)-linux-amd64 \
		-o $(BIN_DIR)/$(PROJECT)-linux-amd64-upx 2>/dev/null || true

# ── 冒烟测试 ────────────────────────────────────────────
smoke: build-linux-amd64
	@echo "=== 编译产物 ==="
	file $(BIN_DIR)/$(PROJECT)-linux-amd64
	@echo ""
	@echo "=== 文件大小 ==="
	ls -lh $(BIN_DIR)/$(PROJECT)-linux-amd64
	@echo ""
	@echo "=== 检查静态链接 ==="
	@ldd $(BIN_DIR)/$(PROJECT)-linux-amd64 2>&1 | grep -q "statically linked" && \
		echo "✓ 静态链接" || echo "✓ 动态链接（需要运行时库）"
	@echo ""
	@echo "=== 版本信息 ==="
	@strings $(BIN_DIR)/$(PROJECT)-linux-amd64 | grep -E "^[0-9]+\.[0-9]+\.[0-9]+" || true
	@echo "OK"

# ── 测试 ─────────────────────────────────────────────────
test:
	go test ./pkg/...

test-verbose:
	go test -v ./pkg/...

# ── 清理 ─────────────────────────────────────────────────
clean:
	rm -rf $(BIN_DIR)/* $(DIST_DIR)/*

# ── 依赖管理 ─────────────────────────────────────────────
deps:
	go mod tidy
	go mod download

fmt:
	go fmt ./pkg/... ./cmd/...

vet:
	go vet ./pkg/... ./cmd/...
