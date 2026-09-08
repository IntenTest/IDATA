//go:build windows && idata_bundle

package executionservice

import _ "embed"

//go:embed runtime.zip
var windowsRuntime []byte

func init() { bundledRuntime = windowsRuntime }
