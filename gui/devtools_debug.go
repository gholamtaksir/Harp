//go:build harpdebug

package main

import "os"

// instanceSuffix keeps debug builds from handing over to an installed Harp.
const instanceSuffix = ".debug"

// browserArgs enables the WebView2 DevTools protocol on the port given in
// HARP_DEVTOOLS_PORT. Only compiled into builds with the harpdebug tag, used
// for automated UI checks; release builds never expose it.
func browserArgs() []string {
	if port := os.Getenv("HARP_DEVTOOLS_PORT"); port != "" {
		return []string{"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=" + port}
	}
	return nil
}
