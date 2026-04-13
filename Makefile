SERVICES := api-gateway chain-gateway scan-account-service scan-utxo-service sign-service wallet-core

.PHONY: build test vet lint vuln fmt tidy proto

build:
	@for svc in $(SERVICES); do \
		echo "building $$svc ..."; \
		go build ./services/$$svc/... || exit 1; \
	done
	@echo "all services built"

test:
	@for svc in $(SERVICES); do \
		echo "testing $$svc ..."; \
		go test ./services/$$svc/... || exit 1; \
	done

vet:
	@for svc in $(SERVICES); do \
		echo "vetting $$svc ..."; \
		go vet ./services/$$svc/... || exit 1; \
	done

fmt:
	gofmt -l -w services/

tidy:
	@for svc in $(SERVICES); do \
		echo "tidy $$svc ..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done

vuln:
	@command -v govulncheck >/dev/null 2>&1 || { echo "install: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	@for svc in $(SERVICES); do \
		echo "vuln-check $$svc ..."; \
		cd services/$$svc && govulncheck ./... && cd ../..; \
	done
	@echo "govulncheck passed"

proto:
	@ROOT=$$(pwd); cd shared/proto && \
	PATH="$$PATH:$$HOME/go/bin" protoc \
	  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/chain-gateway/internal/pb/chaingateway \
	  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/chain-gateway/internal/pb/chaingateway \
	  chaingateway.proto && \
	PATH="$$PATH:$$HOME/go/bin" protoc \
	  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/wallet-core/internal/pb/chaingateway \
	  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/wallet-core/internal/pb/chaingateway \
	  chaingateway.proto && \
	PATH="$$PATH:$$HOME/go/bin" protoc \
	  --go_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/scan-account-service/internal/pb/chaingateway \
	  --go-grpc_out=paths=source_relative,Mchaingateway.proto=wallet-saas-v2/shared/proto/chaingateway\;chaingateway:$$ROOT/services/scan-account-service/internal/pb/chaingateway \
	  chaingateway.proto && \
	echo "proto generated"
