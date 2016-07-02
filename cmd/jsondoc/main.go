// Copyright 2016 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/golang-commonmark/markdown"
)

func main() {
	log.SetFlags(0)
	d, err := NewJSONDoc("../../example", "../../example/index.md")
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(header)
	err = d.WriteTo(os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(footer)
}

const header = `
<html>
<head>
<style>
body {
    margin: 1em;
}
h1, h2, h3, h4 {
    font-family: sans-serif;
}
table {
    border: solid 1px #808080;
    border-collapse: collapse;
    page-break-inside: avoid;
}
td, th {
    border: solid 1px #808080;
    padding: 0.7em;
}
th {
    background-color: #e0e0e0;
}
</style>
</head>
<body>
`

const footer = `
</body>
</html>
`

type JSONDoc struct {
	pkgName  string
	pkg      map[string]*ast.Package
	t        *template.Template
	tmplName string
}

const table = `
<p>JSON object with the following fields:</p>
<table>
<tr>
<th>Key name</th>
<th>Value type</th>
<th>Description</th>
</tr>
{{range .}}
<tr>
<td>{{.Name}}</td>
<td>{{.Type}}</td>
<td>{{.Description}}</td>
</tr>
{{end}}
</table>
`

func NewJSONDoc(pkg, index string) (*JSONDoc, error) {
	d := &JSONDoc{pkgName: pkg}
	fset := token.NewFileSet()
	p, err := parser.ParseDir(fset, pkg, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	d.pkg = p
	d.t = template.New("table").Funcs(template.FuncMap{"input": d.input, "output": d.output})
	if _, err := d.t.Parse(table); err != nil {
		return nil, err
	}
	if _, err := d.t.ParseFiles(index); err != nil {
		return nil, err
	}
	d.tmplName = filepath.Base(index)
	return d, nil
}

func (d *JSONDoc) WriteTo(w io.Writer) error {
	var b bytes.Buffer
	if err := d.t.ExecuteTemplate(&b, d.tmplName, nil); err != nil {
		return err
	}
	md := markdown.New(markdown.HTML(true))
	if err := md.Render(w, b.Bytes()); err != nil {
		return err
	}
	return nil
}

func (d *JSONDoc) input(name string) (string, error) {
	s, err := d.renderType(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<div>\n<h3>Input (%s)</h3>\n%s\n</div>\n", html.EscapeString(name), s), nil
}

func (d *JSONDoc) output(name string) (string, error) {
	s, err := d.renderType(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<div>\n<h3>Output (%s)</h3>\n%s\n</div>\n", html.EscapeString(name), s), nil
}

func (d *JSONDoc) renderType(name string) (string, error) {
	var o *ast.Object
outer:
	for _, pkg := range d.pkg {
		for _, f := range pkg.Files {
			o = f.Scope.Objects[name]
			if o != nil {
				break outer
			}
		}
	}
	if o == nil {
		return "", fmt.Errorf("Type %s not found", name)
	}
	t, ok := o.Decl.(*ast.TypeSpec)
	if !ok {
		return "", fmt.Errorf("Object named %s is not a type", name)
	}

	var b bytes.Buffer
	if t, ok := t.Type.(*ast.StructType); ok {
		type field struct {
			Name, Type, Description string
		}
		var fields []field
		for _, f := range t.Fields.List {
			for _, indent := range f.Names {
				name, err := tagToName(indent.Name, f.Tag)
				if err != nil {
					if err == NotExported {
						continue
					}
					return "", err
				}
				fields = append(fields, field{name, d.typeString(f.Type), strings.TrimSpace(f.Comment.Text())})
			}
		}
		d.t.ExecuteTemplate(&b, "table", fields)
	}
	return b.String(), nil
}

func (d *JSONDoc) typeString(t ast.Expr) string {
	if t, ok := t.(*ast.ArrayType); ok {
		return fmt.Sprintf("array of %s", t.Elt)
	}
	return fmt.Sprint(t)
}

var NotExported = errors.New("Not exported")

func tagToName(name string, tag *ast.BasicLit) (string, error) {
	if !ast.IsExported(name) {
		return "", NotExported
	}
	if tag != nil {
		s, err := strconv.Unquote(tag.Value)
		if err != nil {
			return "", err
		}
		s = reflect.StructTag(s).Get("json")
		if s == "" {
			return strconv.Quote(name), nil
		}
		fields := strings.Split(s, ",")
		if fields[0] == "-" {
			return "", NotExported
		}
		suffix := ""
		for _, f := range fields[1:] {
			if f == "omitempty" {
				suffix = " (optional)"
			}
		}
		return strconv.Quote(fields[0]) + suffix, nil
	}
	return strconv.Quote(name), nil
}
