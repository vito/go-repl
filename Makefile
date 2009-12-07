include $(GOROOT)/src/Make.$(GOARCH)

TARG=repl
GOFILES=\
        main.go

include $(GOROOT)/src/Make.cmd
