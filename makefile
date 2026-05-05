build:
	@go build -o go-commitgen ./cmd/go-commitgen
	@mv go-commitgen ~/go/bin/
	@echo ">> Finished"