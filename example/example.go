// Copyright 2016 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package example

type helloInput struct {
	Name string `json:"test"` // Name to be used in greetings
	A, B int    // Some numeric parameter
	Size size   `json:"size"`
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
	Info      info   `json:"info"` // type with anonymous field
	C         struct {
		A, B string
	}
	F []struct {
		A, B, C int
	} `json:"f"`
}

type size struct {
	Length float64 `json:"lenght"` // length of the object
	Width  float64 `json:"width"`  // width of the object
	Height float64 `json:"height"` // height of the object
}

type info struct {
	size
	Weight float64 `json:"weight"` // weight of the object
}

type empty struct{}
type emptyA []struct{}
type emptyAA [][]struct{}
type emptyO map[string]struct{}
type emptyOO map[string]map[string]struct{}
type emptyAO []map[string]struct{}
type emptyOA map[string][]struct{}

type mapInput map[string]map[string][]int

type mapOutput map[string]struct {
	I    int
	OI   map[string]struct{ I int }
	OOAI map[string]map[string][]struct{ I int }
}

type arrayInput [][]struct {
	AS  []string
	AAI [][]int
}
