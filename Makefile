install:
	go build -o ./.bin/client ./pkg/client \
	&& go build -o ./.bin/clock ./pkg/clock