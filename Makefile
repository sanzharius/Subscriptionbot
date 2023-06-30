include .env

.PHONY: all build run

all: build run

build:
	docker build -t main .

run:
	docker-compose -f docker-compose.yml up

stop:
	docker-compose -f docker-compose.yml stop

