.PHONY: generate generate-backend generate-frontend

generate: generate-backend generate-frontend

generate-backend:
	cd backend && go generate ./...

generate-frontend:
	cd frontend && npx orval
