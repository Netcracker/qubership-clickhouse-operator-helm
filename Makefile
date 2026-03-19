.PHONY: lint

lint:
	@for dir in $$(find . -name "go.mod" -exec dirname {} \;); do \
		echo "=== $$dir ==="; \
		(cd "$$dir" && golangci-lint run ./...); \
	done
