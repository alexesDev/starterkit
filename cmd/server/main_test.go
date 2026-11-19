package main

import (
	"testing"

	"go.uber.org/goleak"
)

const goqiteExtendSleeper = "maragu.dev/goqite/jobs.(*Runner).receiveAndRun.func1.3"

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction(goqiteExtendSleeper))
}
