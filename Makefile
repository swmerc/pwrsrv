#
# Build time variables
#

# Moderately nice versioning.  I guess.
TAG := $(shell git describe HEAD)

BRANCH := -$(shell git branch --show-current)
ifeq ($(BRANCH),-main)
    BRANCH :=
endif

ifneq ($(shell git status --porcelain),)
    DIRTY := -dirty
endif

FLAGS := -ldflags "-X main.version=$(TAG)$(BRANCH)$(DIRTY)"

#
# The rest is just boilerplate cross platform
#

.PHONY: all
all: native arm amd64

.PHONY: clean
clean:
	rm builds/*

.PHONY: native
native:
	go build $(FLAGS) -o builds/native/pwrsrv

.PHONY: arm
arm:
	GOOS=linux GOARCH=arm GOARM=5 go build $(FLAGS) -o builds/arm/pwrsrv

.PHONY: amd64
amd64:
	GOOS=linux GOARCH=amd64 go build $(FLAGS) -o builds/amd64/pwrsrv
