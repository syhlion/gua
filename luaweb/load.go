package luaweb

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
	lua "github.com/yuin/gopher-lua"
)

func Map2table(L *lua.LState, m map[string]string) *lua.LTable {
	table := L.NewTable()
	for key, value := range m {
		L.RawSet(table, lua.LString(key), lua.LString(value))
	}
	return table
}

func Flush(w http.ResponseWriter) bool {
	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
	return ok
}

func Load(w http.ResponseWriter, req *http.Request, L *lua.LState, log *logrus.Logger) {

	// Print text to the web page that is being served. Add a newline.
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		var buf bytes.Buffer
		top := L.GetTop()
		for i := 1; i <= top; i++ {
			buf.WriteString(L.Get(i).String())
			if i != top {
				buf.WriteString("\t")
			}
		}
		// Final newline
		buf.WriteString("\n")

		// Write the combined text to the http.ResponseWriter
		buf.WriteTo(w)

		return 0 // number of results
	}))
	// Flush the ResponseWriter.
	// Needed in debug mode, where ResponseWriter is buffered.
	L.SetGlobal("flush", L.NewFunction(func(L *lua.LState) int {
		Flush(w)
		return 0 // number of results
	}))

	// Set the Content-Type for the page
	L.SetGlobal("content", L.NewFunction(func(L *lua.LState) int {
		lv := L.ToString(1)
		w.Header().Add("Content-Type", lv)
		return 0 // number of results
	}))

	// Return the current URL Path
	L.SetGlobal("urlpath", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(req.URL.Path))
		return 1 // number of results
	}))

	// Return the current HTTP method (GET, POST etc)
	L.SetGlobal("method", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(req.Method))
		return 1 // number of results
	}))

	// Return the HTTP headers as a table
	L.SetGlobal("headers", L.NewFunction(func(L *lua.LState) int {
		luaTable := L.NewTable()
		for key := range req.Header {
			L.RawSet(luaTable, lua.LString(key), lua.LString(req.Header.Get(key)))
		}
		if req.Host != "" {
			L.RawSet(luaTable, lua.LString("Host"), lua.LString(req.Host))
		}
		L.Push(luaTable)
		return 1 // number of results
	}))

	// Return the HTTP header in the request, for a given key/string
	L.SetGlobal("header", L.NewFunction(func(L *lua.LState) int {
		key := L.ToString(1)
		value := req.Header.Get(key)
		L.Push(lua.LString(value))
		return 1 // number of results
	}))

	// Set the HTTP header in the request, for a given key and value
	L.SetGlobal("setheader", L.NewFunction(func(L *lua.LState) int {
		key := L.ToString(1)
		value := L.ToString(2)
		w.Header().Set(key, value)
		return 0 // number of results
	}))

	// Return the HTTP body in the request
	L.SetGlobal("body", L.NewFunction(func(L *lua.LState) int {
		body, err := ioutil.ReadAll(req.Body)
		var result lua.LString
		if err != nil {
			result = lua.LString("")
		} else {
			result = lua.LString(string(body))
		}
		L.Push(result)
		return 1 // number of results
	}))

	// Set the HTTP status code (must come before print)
	L.SetGlobal("status", L.NewFunction(func(L *lua.LState) int {
		code := int(L.ToNumber(1))
		w.WriteHeader(code)
		return 0 // number of results
	}))

	// Set a HTTP status code and print a message (optional)
	L.SetGlobal("error", L.NewFunction(func(L *lua.LState) int {
		code := int(L.ToNumber(1))
		w.WriteHeader(code)
		if L.GetTop() == 2 {
			message := L.ToString(2)
			fmt.Fprint(w, message)
		}
		return 0 // number of results
	}))

	// Retrieve a table with keys and values from the form in the request
	L.SetGlobal("formdata", L.NewFunction(func(L *lua.LState) int {
		// Place the form data in a map
		m := make(map[string]string)
		req.ParseForm()
		for key, values := range req.Form {
			m[key] = values[0]
		}
		// Convert the map to a table and return it
		L.Push(Map2table(L, m))
		return 1 // number of results
	}))

	// Retrieve a table with keys and values from the URL in the request
	L.SetGlobal("urldata", L.NewFunction(func(L *lua.LState) int {

		var valueMap url.Values
		var err error

		if L.GetTop() == 1 {
			// If given an argument
			rawurl := L.ToString(1)
			valueMap, err = url.ParseQuery(rawurl)
			// Log error as warning if there are issues.
			// An empty Value map will then be used.
			if err != nil {
				log.Error(err)
				// return 0
			}
		} else {
			// If not given an argument
			valueMap = req.URL.Query() // map[string][]string
		}

		// Place the Value data in a map, using the first values
		// if there are many values for a given key.
		m := make(map[string]string)
		for key, values := range valueMap {
			m[key] = values[0]
		}
		// Convert the map to a table and return it
		L.Push(Map2table(L, m))
		return 1 // number of results
	}))

	// Redirect a request (as found, by default)
	L.SetGlobal("redirect", L.NewFunction(func(L *lua.LState) int {
		newurl := L.ToString(1)
		httpStatusCode := http.StatusFound
		if L.GetTop() == 2 {
			httpStatusCode = int(L.ToNumber(2))
		}
		http.Redirect(w, req, newurl, httpStatusCode)
		return 0 // number of results
	}))

	// Permanently redirect a request, which is the same as redirect(url, 301)
	L.SetGlobal("permanent_redirect", L.NewFunction(func(L *lua.LState) int {
		newurl := L.ToString(1)
		httpStatusCode := http.StatusMovedPermanently
		http.Redirect(w, req, newurl, httpStatusCode)
		return 0 // number of results
	}))

}
