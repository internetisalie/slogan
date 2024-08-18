package errors

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

var ErrTest = NewSentinel("globally-defined error")

func newError(text string) error {
	return newSimple(text, nil)
}

func wrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return newSimple(message, err)
}

func recursive(count int, fn func()) {
	if count > 1 {
		recursive(count-1, fn)
	} else {
		fn()
	}
}

func TestCallersPage(t *testing.T) {
	const frames = 24
	const callers = 3
	recursive(frames, func() {
		page := newCallersPage(1) // skip this function

		var frame uintptr
		var ok bool
		var count int
		frame, page, ok = page.Next()
		for ok {
			count++
			frame, page, ok = page.Next()
		}

		assert.Zero(t, frame)
		assert.Equal(t, frames+callers, count)
	})
}

func TestCallers(t *testing.T) {
	const frameCount = 24
	const callerCount = 3
	recursive(frameCount, func() {
		c := newCallers(1) // skip this function

		var frame uintptr
		var ok bool
		var count int
		frame, c, ok = c.Next()
		for ok {
			count++
			frame, c, ok = c.Next()
		}

		assert.Zero(t, frame)
		assert.Equal(t, count, frameCount+callerCount)
	})
}

func TestStackTrimmer(t *testing.T) {
	const frameCount = 5
	const callerCount = 3

	var err error

	recursive(frameCount, func() {
		recursive(frameCount, func() {
			err = newError("inner")
		})
		err = wrapError(err, "outer")
	})

	var outerStacker Stacker
	assert.ErrorAs(t, err, &outerStacker)
	assert.Len(t, outerStacker.Stack(), frameCount+callerCount+1) // add inner anonymous function
	if len(outerStacker.Stack()) != frameCount+callerCount+1 {
		spew.Dump(outerStacker.Stack())
	}

	var outerUnwrapper Unwrapper
	assert.ErrorAs(t, err, &outerUnwrapper)

	innerError := outerUnwrapper.Unwrap()
	assert.Error(t, innerError)

	var innerStacker Stacker
	assert.ErrorAs(t, innerError, &innerStacker)
	assert.Len(t, innerStacker.Stack(), frameCount+1+1) // add inner function and outer statement
	if len(innerStacker.Stack()) != frameCount+1+1 {
		spew.Dump(innerStacker.Stack())
	}
}

func TestStackTrimmer_Global(t *testing.T) {
	const frameCount = 5
	const callerCount = 3
	const initCount = 5

	var err error

	recursive(frameCount, func() {
		err = wrapError(ErrTest, "wrapper")
	})

	var outerStacker Stacker
	assert.ErrorAs(t, err, &outerStacker)
	assert.Len(t, outerStacker.Stack(), frameCount+callerCount+1) // add inner anonymous function

	var outerUnwrapper Unwrapper
	assert.ErrorAs(t, err, &outerUnwrapper)

	innerError := outerUnwrapper.Unwrap()
	assert.Error(t, innerError)

	var innerStacker Stacker
	assert.ErrorAs(t, innerError, &innerStacker)
	assert.Len(t, innerStacker.Stack(), 0)
	if len(innerStacker.Stack()) != 0 {
		spew.Dump(innerStacker.Stack())
	}
}
