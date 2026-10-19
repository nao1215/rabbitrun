//go:build js

package save

import (
	"encoding/json"
	"syscall/js"
	"testing"
)

// These tests run the browser's save data under Node (GOOS=js GOARCH=wasm go test with
// go_js_wasm_exec); each one puts a localStorage of its own on the global object.

// setStorage makes the global localStorage the object the JavaScript body returns.
func setStorage(t *testing.T, body string) {
	t.Helper()
	obj := js.Global().Get("Function").New(body).Invoke()
	js.Global().Get("Object").Call("defineProperty", js.Global(), "localStorage",
		map[string]any{"value": obj, "configurable": true, "writable": true})
}

// blockStorage makes reading the global localStorage throw, as it does in a browser that
// blocks storage for the site.
func blockStorage(t *testing.T) {
	t.Helper()
	js.Global().Get("Function").New(`Object.defineProperty(globalThis, 'localStorage',
		{get() { throw new Error('SecurityError') }, configurable: true})`).Invoke()
}

// memoryStorage is a working localStorage kept in a Map; failSet makes setItem throw as a
// full quota does.
func memoryStorage(t *testing.T, failSet bool) {
	t.Helper()
	set := "m.set(k, String(v))"
	if failSet {
		set = "throw new Error('QuotaExceededError')"
	}
	setStorage(t, `const m = new Map(); return {
		getItem: k => m.has(k) ? m.get(k) : null,
		setItem: (k, v) => { `+set+` },
		removeItem: k => { m.delete(k) },
		_map: m,
	};`)
}

// stored is the value of key in the test's localStorage, and whether it is set.
func stored(key string) (string, bool) {
	v := js.Global().Get("localStorage").Get("_map").Call("get", key)
	if v.Type() != js.TypeString {
		return "", false
	}
	return v.String(), true
}

//nolint:paralleltest // shares the global localStorage
func TestLocalStorageRoundTrip(t *testing.T) {
	memoryStorage(t, false)
	st := NewStore()
	st.Load()
	if st.kept {
		t.Fatal("a first launch (no save data) must not stop the game from saving")
	}
	st.Data.Progress("gyal").BestStage = 3
	st.Mark()
	st.Flush()
	raw, ok := stored(Path())
	if !ok {
		t.Fatal("the save data was not written to localStorage")
	}
	want, err := json.MarshalIndent(st.Data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatalf("the stored JSON differs from the desktop's save file:\n%s\nwant\n%s", raw, want)
	}
	again := NewStore()
	again.Load()
	if got := again.Data.Progress("gyal").BestStage; got != 3 {
		t.Fatalf("best stage after loading = %d, want 3", got)
	}
}

//nolint:paralleltest // shares the global localStorage
func TestLocalStorageKeepsBrokenData(t *testing.T) {
	memoryStorage(t, false)
	if err := setItem(Path(), `{"characters":{"gyal":`); err != nil {
		t.Fatal(err)
	}
	st := NewStore()
	st.Load()
	if got, ok := stored(Path() + ".broken"); !ok || got != `{"characters":{"gyal":` {
		t.Fatalf("the broken save data was not kept aside: %q", got)
	}
	if st.kept {
		t.Fatal("broken save data that was kept aside must not stop the game from saving")
	}
}

//nolint:paralleltest // shares the global localStorage
func TestLocalStorageReset(t *testing.T) {
	memoryStorage(t, false)
	if moved, err := Reset(); err != nil || moved {
		t.Fatalf("Reset without save data = %v, %v; want false, nil", moved, err)
	}
	if err := setItem(Path(), `{"characters":{}}`); err != nil {
		t.Fatal(err)
	}
	if moved, err := Reset(); err != nil || !moved {
		t.Fatalf("Reset = %v, %v; want true, nil", moved, err)
	}
	if _, ok := stored(Path()); ok {
		t.Fatal("the save data is still there after Reset")
	}
	if got, ok := stored(Path() + ".bak"); !ok || got != `{"characters":{}}` {
		t.Fatalf("the old save data was not kept as .bak: %q", got)
	}
}

// TestNoLocalStorage: a page without localStorage, or one that throws when it is touched
// (a private window, storage blocked for the site), plays on without saving.
//
//nolint:paralleltest // shares the global localStorage
func TestNoLocalStorage(t *testing.T) {
	for name, set := range map[string]func(*testing.T){
		"missing": func(t *testing.T) { t.Helper(); setStorage(t, "return undefined") },
		"blocked": blockStorage,
		"throws": func(t *testing.T) {
			t.Helper()
			setStorage(t, `const no = () => { throw new Error('SecurityError') };
				return {getItem: no, setItem: no, removeItem: no};`)
		},
	} {
		t.Run(name, func(t *testing.T) { //nolint:paralleltest // shares the global localStorage
			set(t)
			st := NewStore()
			st.Load()
			if !st.kept {
				t.Fatal("the store must not try to write where it cannot read")
			}
			st.Data.Progress("cool").Cleared = true
			st.Mark()
			st.Flush() // must not panic
			if st.dirty {
				t.Fatal("the change stays pending: Flush would try again every frame")
			}
			if moved, err := Reset(); err == nil || moved {
				t.Fatalf("Reset = %v, %v; want an error", moved, err)
			}
		})
	}
}

// TestFullLocalStorageIsRetried: a write that throws (a full quota) keeps the change
// pending, as a failed write of the save file does.
//
//nolint:paralleltest // shares the global localStorage
func TestFullLocalStorageIsRetried(t *testing.T) {
	memoryStorage(t, true)
	st := NewStore()
	st.Load()
	st.Mark()
	st.Flush()
	if !st.dirty || !st.failing {
		t.Fatal("a failed write must stay pending to be tried again")
	}
}
