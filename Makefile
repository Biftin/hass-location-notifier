go_files := $(shell find . -type f -name '*.go')

hass-location-notifier: $(go_files)
	go build -o $@

.PHONY: docker
docker:
	docker build -t hass-location-notifier .

.PHONY: clean
clean:
	rm hass-location-notifier*
