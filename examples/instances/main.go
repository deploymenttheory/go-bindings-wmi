//go:build windows

// Command instances demonstrates the instance CRUD surface — create, read
// (generated key lookup + read-by-path), update, delete — using
// Win32_Environment, a harmless user-scoped writable class.
//
//	go run ./examples/instances
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		fmt.Println("connect:", err)
		return
	}
	defer svc.Close()

	const name = "GO_BINDINGS_WMI_DEMO"
	userName := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	path := fmt.Sprintf(`Win32_Environment.Name=%q,UserName=%q`, name, userName)
	defer svc.DeleteInstance(path) // self-cleaning demo

	// C — create a per-user environment variable.
	created, err := svc.CreateInstance("Win32_Environment", map[string]any{
		"Name":          name,
		"UserName":      userName,
		"VariableValue": "hello from go-bindings-wmi",
	})
	if err != nil {
		fmt.Println("create:", err)
		return
	}
	fmt.Println("created:", created)

	// R — the generated key lookup (Name + UserName are Win32_Environment's
	// [key] properties), and the runtime read-by-path.
	env, err := cimv2.GetWin32Environment(svc, name, userName)
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	fmt.Printf("read:    %s = %q\n", env.Name, env.VariableValue)

	// U — change the value in place.
	if err := svc.UpdateInstance(path, map[string]any{"VariableValue": "updated"}); err != nil {
		fmt.Println("update:", err)
		return
	}
	env, _ = cimv2.GetWin32Environment(svc, name, userName)
	fmt.Printf("updated: %s = %q\n", env.Name, env.VariableValue)

	// D — delete, then show ErrNotFound from both read paths.
	if err := svc.DeleteInstance(path); err != nil {
		fmt.Println("delete:", err)
		return
	}
	if _, err := cimv2.GetWin32Environment(svc, name, userName); errors.Is(err, wmi.ErrNotFound) {
		fmt.Println("deleted: lookup now reports wmi.ErrNotFound")
	}
}
