.PHONY: deploy
deploy: build
	rsync -avz --info=progress2 server/katherineandchandler.com weddingsite@cerium.chandlerswift.com:katherineandchandler.com
	ssh cerium.chandlerswift.com sudo systemctl restart weddingsite

.PHONY: build
build:
	cd hugo_site && hugo
	cd server && go build -ldflags="-extldflags=-static" -tags netgo,sqlite_omit_load_extension .
