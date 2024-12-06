GOCMD ?= go

.PHONY: tidy
tidy:
	$(GOCMD) mod tidy

.PHONY: lint-frontend
lint-frontend:
	npm run lint

.PHONY: lint-frontend-fix
lint-frontend-fix:
	npm run lint:fix

.PHONY: lint-go tidy
lint-go:
	golangci-lint run

.PHONY: lint-go-fix
lint-go-fix:
	golangci-lint run --fix

.PHONY: lint
lint: lint-go lint-frontend

.PHONY: lint-fix
lint-fix: lint-go-fix lint-frontend-fix

node_modules: package-lock.json
	npm install --no-save
	@touch node_modules

.PHONY: frontend-dev
frontend-dev: node_modules
	npm run build:dev

.PHONY: frontend-prod
frontend-prod: node_modules
	npm run build:prod

.PHONY: run
run: frontend-dev tidy
	$(GOCMD) run ./cmd/

.PHONY: build
build: frontend-prod tidy
	$(GOCMD) build -o dist/cloud ./cmd/

.PHONY: docker-build
docker-build:
	docker build . --tag cloud  --network host

.PHONY: init-db
init-db:
	 cat sql/init_db.sql sql/test_data.sql | docker compose exec -T postgres psql --dbname glasskube --user local
