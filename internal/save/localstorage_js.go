//go:build js

package save

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall/js"
)

// The save data of the browser game: the same JSON as the desktop's save file, kept in
// the page's localStorage under the key Path. The desktop's save.json.bak and
// save.json.broken are the keys Path()+".bak" and Path()+".broken".
//
// localStorage may be missing or refuse to work (a private window, storage blocked for
// the site, the quota used up): it then throws, which syscall/js turns into a panic.
// Every use goes through call, which turns that back into an error, so the game plays on
// without saving: Load finds an error that is not fs.ErrNotExist and keeps the store from
// writing, and a write that fails is retried by Flush as on the desktop.

// storageKey is the localStorage key of the save data.
const storageKey = "rabbitrun/save.json"

// errNoStorage is returned when the page has no localStorage at all.
var errNoStorage = errors.New("this browser keeps no localStorage for the page")

// Path is where the save data is kept: its localStorage key.
func Path() string { return storageKey }

// call calls localStorage.method(args...), returning what it throws as an error.
func call(method string, args ...any) (v js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage.%s: %v", method, r)
		}
	}()
	// Got through Reflect.get, as a call: a getter that throws in a plain Get is not
	// caught, and ends the whole program.
	ls := js.Global().Get("Reflect").Call("get", js.Global(), "localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return js.Undefined(), errNoStorage
	}
	return ls.Call(method, args...), nil
}

// getItem returns the value of key, or an error wrapping fs.ErrNotExist when it is not set.
func getItem(key string) (string, error) {
	v, err := call("getItem", key)
	if err != nil {
		return "", err
	}
	if v.Type() != js.TypeString {
		return "", fs.ErrNotExist
	}
	return v.String(), nil
}

func setItem(key, value string) error {
	_, err := call("setItem", key, value)
	return err
}

func removeItem(key string) error {
	_, err := call("removeItem", key)
	return err
}

// Reset starts the save data over: the save data is moved aside to the key
// Path()+".bak". It reports whether there was save data to move.
func Reset() (bool, error) {
	raw, err := getItem(storageKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := setItem(storageKey+".bak", raw); err != nil {
		return false, err
	}
	return true, removeItem(storageKey)
}

// readSave reads the save data; the error wraps fs.ErrNotExist when there is none yet.
func readSave() ([]byte, error) {
	raw, err := getItem(storageKey)
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

// keepBroken copies the save data that did not decode, raw, to the key Path()+".broken".
func keepBroken(raw []byte) error { return setItem(storageKey+".broken", string(raw)) }

// writeSave writes raw as the save data. setItem replaces the value whole or not at all,
// so a failure leaves the old save as it was.
func writeSave(raw []byte) error { return setItem(storageKey, string(raw)) }
