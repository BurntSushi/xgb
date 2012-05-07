# This Makefile is used by the developer. It is not needed in any way to build
# a checkout of the XGB repository.
# It will be useful, however, if you are hacking at the code generator.

XPROTO=/usr/share/xcb

# All of the XML files in my /usr/share/xcb directory EXCEPT XKB. -_-
all: bigreq.xml composite.xml damage.xml dpms.xml dri2.xml \
		 ge.xml glx.xml randr.xml record.xml render.xml res.xml \
		 screensaver.xml shape.xml shm.xml sync.xml xc_misc.xml \
		 xevie.xml xf86dri.xml xf86vidmode.xml xfixes.xml xinerama.xml \
		 xinput.xml xprint.xml xproto.xml xselinux.xml xtest.xml \
		 xvmc.xml xv.xml

%.xml:
	xgbgen/xgbgen --proto-path $(XPROTO) $(XPROTO)/$*.xml > auto_$*.go

test:
	go test

bench:
	go test -run 'nomatch' -bench '.*' -cpu 1,2,6

gofmt:
	gofmt -w *.go xgbgen/*.go examples/*.go examples/*/*.go
	colcheck xgbgen/*.go examples/*.go examples/*/*.go \
					 auth.go conn.go cookie.go doc.go xgb.go xgb_help.go xgb_test.go

