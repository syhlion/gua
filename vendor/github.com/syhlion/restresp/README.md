# rest response

[![Go Report Card](https://goreportcard.com/badge/github.com/syhlion/restresp)](https://goreportcard.com/report/github.com/syhlion/restresp)
[![Build Status](https://travis-ci.org/syhlion/restresp.svg?branch=master)](https://travis-ci.org/syhlion/restresp)

## Install

`go get -u github.com/syhlion/restresp`

## Usege

```
type customError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (c *customError) Error() string {
	return c.Msg
}

func testHandler(w http.ResponseWriter, r *http.Request) {

	d := &customError{5, "wtf"}
	restresp.Write(w, d, 200)
}
```

