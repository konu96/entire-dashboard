.PHONY: generate generate-sqlc generate-backend generate-frontend

generate: generate-sqlc generate-backend generate-frontend

generate-sqlc:
	cd backend && sqlc generate

generate-backend:
	cd backend && go generate ./...

generate-frontend:
	cd frontend && npm run generate
