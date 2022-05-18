package main

import (
	"testing"

	"github.com/ysmood/gotrace"
)

func TestBrowserPool(t *testing.T) {
	// Just add one line before the standard test case
	gotrace.CheckLeak(t, 0)

	BrowserPool("../../urls.txt")
}
