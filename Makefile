# This Makefile is used by the developer. It is not needed in any way to build
# a checkout of the XGB repository.
# It will be useful, however, if you are hacking at the code generator.

XPROTO=/usr/share/xcb

# All of the XML files in my /usr/share/xcb directory EXCEPT XKB. -_-
all: build-xgbgen \
		 bigreq.xml composite.xml damage.xml dpms.xml dri2.xml \
		 ge.xml glx.xml randr.xml record.xml render.xml res.xml \
		 screensaver.xml shape.xml shm.xml sync.xml xc_misc.xml \
		 xevie.xml xf86dri.xml xf86vidmode.xml xfixes.xml xinerama.xml \
		 xinput.xml xprint.xml xproto.xml xselinux.xml xtest.xml \
		 xvmc.xml xv.xml

build-xgbgen:
	(cd xgbgen && go build)

build-all: bigreq.b composite.b damage.b dpms.b dri2.b ge.b glx.b randr.b \
					 record.b render.b res.b screensaver.b shape.b shm.b sync.b xcmisc.b \
					 xevie.b xf86dri.b xf86vidmode.b xfixes.b xinerama.b xinput.b \
					 xprint.b xproto.b xselinux.b xtest.b xv.b xvmc.b

%.b:
	(cd $* ; go build)

xc_misc.xml: build-xgbgen
	mkdir -p xcmisc
	xgbgen/xgbgen --proto-path $(XPROTO) $(XPROTO)/xc_misc.xml > xcmisc/xcmisc.go

%.xml: build-xgbgen
	mkdir -p $*
	xgbgen/xgbgen --proto-path $(XPROTO) $(XPROTO)/$*.xml > $*/$*.go

test:
	(cd xproto ; go test)

bench:
	(cd xproto ; go test -run 'nomatch' -bench '.*' -cpu 1,2,6)

gofmt:
	gofmt -w *.go xgbgen/*.go examples/*.go examples/*/*.go
	colcheck xgbgen/*.go examples/*.go examples/*/*.go xproto/xproto_test.go \
					 auth.go conn.go cookie.go doc.go xgb.go xgb_help.go

