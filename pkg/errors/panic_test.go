package errors

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func TestPanicError(t *testing.T) {
	const frameCount = 5
	const anonymousCount = 1
	const panicCount = 1
	const callerCount = 1

	var err error
	done := make(chan struct{})

	go func() {
		recursive(frameCount, func() {
			defer func() {
				if e := recover(); e != nil {
					err = NewPanicError(e, -1)
				}
			}()

			panic(123)
		})

		close(done)
	}()

	<-done

	assert.Error(t, err)

	var stacker Stacker
	assert.ErrorAs(t, err, &stacker)

	s := stacker.Stack()
	assert.Len(t, s, frameCount+callerCount+anonymousCount+panicCount)
	if len(s) != frameCount+callerCount+anonymousCount+panicCount {
		spew.Dump(s)
	}
	spew.Dump(s.LogValue().Any())

	var valueError ValueError
	assert.ErrorAs(t, err, &valueError)
	assert.Equal(t, valueError.Value(), 123)
}
