install:
	go build -o ./.bin/client ./pkg/client \
	&& go build -o ./.bin/clock ./pkg/clock \
	&& go build -o ./.bin/reverb ./pkg/reverb