// Copyright 2016 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package example

type helloInput struct {
	Name string `json:"test"` // Name to be used in greetings
	A, B int    // Some numeric parameter
}

type helloOutput struct {
	Msg string `json:"msg"` // Greetings message for the provided name
}

// indexInput specifies input for /item/get request
type itemGetInput struct {
	ID int64    `json:"id"` // ID of the requested item
	A  string   `json:"a"`  // A represents something
	B  []string `json:"b"`
	C  struct {
		D, E int
		F    []struct {
			A, B int
		} `json:"f"`
		G [][]struct {
			A, B, C int
		} `json:"g"`
	}
	NotExported string `json:"-"`
}

// itemGetOutput specifies output of /item/get request
type itemGetOutput struct {
	RequestID string `json:"request_id"`      // request ID assigned by the server
	Error     string `json:"error,omitempty"` // only present if there was an error
	Name      string `json:"name"`
	Size      size   `json:"size"`
}

type size struct {
	Length float64 `json:"lenght"` // length of the object
	Width  float64 `json:"width"`  // width of the object
	Height float64 `json:"height"` // height of the object
}
