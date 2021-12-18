.PHONY: deploy
deploy: build
	rsync -avz --delete public/ cerium:/var/www/katherineandchandler.com/

.PHONY: build
build:
	hugo
