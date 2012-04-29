XPROTO=/usr/share/xcb
all: xproto xinerama

xproto:
	python2 go_client.py $(XPROTO)/xproto.xml
	gofmt -w xproto.go

xinerama:
	python2 go_client.py $(XPROTO)/xinerama.xml
	gofmt -w xinerama.go

randr:
	python2 go_client.py $(XPROTO)/randr.xml
	gofmt -w randr.go

render:
	python2 go_client.py $(XPROTO)/render.xml
	gofmt -w render.go

