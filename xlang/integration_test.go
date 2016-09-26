package xlang_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"sourcegraph.com/sourcegraph/sourcegraph/pkg/lsp"
	"sourcegraph.com/sourcegraph/sourcegraph/xlang"
	"sourcegraph.com/sourcegraph/sourcegraph/xlang/uri"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip long integration test")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skip network-dependent test in CI")
	}

	tests := map[string]struct { // map key is rootPath
		mode           string
		wantHover      map[string]string
		wantDefinition map[string]string
	}{
		"git://github.com/gorilla/mux?0a192a193177452756c362c20087ddafcf6829c4": {
			mode: "go",
			wantHover: map[string]string{
				"mux.go:61:38": "type Request struct{Method string; URL *URL; Proto string...", // stdlib
			},
			wantDefinition: map[string]string{
				"mux.go:61:38": "git://github.com/golang/go?go1.7.1#src/net/http/request.go:76:6", // stdlib
			},
		},
		"git://github.com/coreos/fuze?7df4f06041d9daba45e4c68221b9b04203dff1d8": {
			mode: "go",
			wantHover: map[string]string{
				"config/convert.go:262:26": "func ParseBase2Bytes(s string) (Base2Bytes, error)", // vendored
			},
			wantDefinition: map[string]string{
				"config/convert.go:262:26": "git://github.com/coreos/fuze?7df4f06041d9daba45e4c68221b9b04203dff1d8#config/vendor/github.com/alecthomas/units/bytes.go:30:6", // vendored TODO(sqs): really want the below result which has the non-vendored path as well, need to implement that
				//"config/convert.go:262:26": "git://github.com/coreos/fuze?7df4f06041d9daba45e4c68221b9b04203dff1d8#config/vendor/github.com/alecthomas/units/bytes.go:30:6 git://github.com/alecthomas/units#bytes.go:30:6", // vendored
			},
		},
		"git://github.com/golang/lint?c7bacac2b21ca01afa1dee0acf64df3ce047c28f": {
			mode: "go",
			wantHover: map[string]string{
				"golint/golint.go:91:18": "type Linter struct{}", // diff pkg, same repo
			},
			wantDefinition: map[string]string{
				"golint/golint.go:91:18": "git://github.com/golang/lint?c7bacac2b21ca01afa1dee0acf64df3ce047c28f#lint.go:31:6", // diff pkg, same repo
			},
		},
		"git://github.com/gorilla/csrf?a8abe8abf66db8f4a9750d76ba95b4021a354757": {
			mode: "go",
			wantHover: map[string]string{
				"csrf.go:57:28": "type SecureCookie struct{...", // diff repo
			},
			wantDefinition: map[string]string{
				"csrf.go:57:28": "git://github.com/gorilla/securecookie?HEAD#securecookie.go:154:6", // diff repo
			},
		},
		"git://github.com/golang/go?go1.7.1": {
			mode: "go",
			wantHover: map[string]string{
				"src/encoding/hex/hex.go:70:12":  "func fromHexChar(c byte) (byte, bool)", // func decl
				"src/encoding/hex/hex.go:104:18": "type Buffer struct{...",                // bytes.Buffer
				"src/net/http/server.go:78:32":   "type Request struct{...",
			},
			wantDefinition: map[string]string{
				"src/encoding/hex/hex.go:70:12":  "git://github.com/golang/go?go1.7.1#src/encoding/hex/hex.go:70:6", // func decl
				"src/encoding/hex/hex.go:104:18": "git://github.com/golang/go?go1.7.1#src/bytes/buffer.go:17:6",     // stdlib type
			},
		},
		"git://github.com/docker/machine?e1a03348ad83d8e8adb19d696bc7bcfb18ccd770": {
			mode: "go",
			wantHover: map[string]string{
				"libmachine/provision/provisioner.go:107:50": "func RunSSHCommandFromDriver(...",
			},
			wantDefinition: map[string]string{
				"libmachine/provision/provisioner.go:107:50": "git://github.com/docker/machine?e1a03348ad83d8e8adb19d696bc7bcfb18ccd770#libmachine/drivers/utils.go:36:6",
			},
		},
	}
	for rootPath, test := range tests {
		label := strings.TrimPrefix(strings.Replace(strings.Replace(rootPath, "//", "", 1), "/", "-", -1), "git:") // abbreviated label
		t.Run(label, func(t *testing.T) {
			ctx := context.Background()
			proxy := xlang.NewProxy()
			addr, done := startProxy(t, proxy)
			defer done()
			c := dialProxy(t, addr, nil)

			// Prepare the connection.
			if err := c.Call(ctx, "initialize", xlang.ClientProxyInitializeParams{
				InitializeParams: lsp.InitializeParams{RootPath: rootPath},
				Mode:             test.mode,
			}, nil); err != nil {
				t.Fatal("initialize:", err)
			}

			root, err := uri.Parse(rootPath)
			if err != nil {
				t.Fatal(err)
			}

			for pos, want := range test.wantHover {
				t.Run(fmt.Sprintf("hover-%s", strings.Replace(pos, "/", "-", -1)), func(t *testing.T) {
					hoverTest(t, ctx, c, root, pos, want)
				})
			}

			for pos, want := range test.wantDefinition {
				t.Run(fmt.Sprintf("definition-%s", strings.Replace(pos, "/", "-", -1)), func(t *testing.T) {
					definitionTest(t, ctx, c, root, pos, want)
				})
			}
		})
	}
}
