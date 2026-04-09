UI_PROJECT := ../doctor/printingpress
BINARY := printing-press

.DEFAULT_GOAL := build

.PHONY: build build-ui templ

build: templ build-ui
	go build -o $(BINARY) printing-press.go

templ:
	$(MAKE) -C $(UI_PROJECT) templ

build-ui:
	$(MAKE) -C $(UI_PROJECT) deps
	$(MAKE) -C $(UI_PROJECT) build-ui
