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
	pkgName     string
	pkg         map[string]*ast.Package
	t           *template.Template
	tmplName    string
	b           bytes.Buffer
	rendered    map[string]struct{}
	renderQueue []*ast.TypeSpec
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
	d := &JSONDoc{pkgName: pkg, rendered: make(map[string]struct{})}
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
	d.b.Reset()
	fmt.Fprintf(&d.b, "<div>\n<h3>Input (%s)</h3>\n", html.EscapeString(name))
	d.rendered[name] = struct{}{}
	if err := d.renderTypes(name); err != nil {
		return "", err
	}
	d.b.WriteString("</div>\n")
	return d.b.String(), nil
}

func (d *JSONDoc) output(name string) (string, error) {
	d.b.Reset()
	fmt.Fprintf(&d.b, "<div>\n<h3>Output (%s)</h3>\n", html.EscapeString(name))
	d.rendered[name] = struct{}{}
	if err := d.renderTypes(name); err != nil {
		return "", err
	}
	d.b.WriteString("</div>\n")
	return d.b.String(), nil
}

func (d *JSONDoc) renderTypes(name string) error {
	if err := d.renderTypeByName(name); err != nil {
		return err
	}
	for i := 0; i < len(d.renderQueue); i++ {
		t := d.renderQueue[i]
		fmt.Fprintf(&d.b, "<h4>Type %s</h4>\n", html.EscapeString(t.Name.Name))
		if err := d.renderType(t); err != nil {
			return err
		}
	}
	d.renderQueue = d.renderQueue[:0]
	return nil
}

func (d *JSONDoc) renderTypeByName(name string) error {
	o := d.findObject(name)
	if o == nil {
		return fmt.Errorf("Type %s not found", name)
	}
	t, ok := o.Decl.(*ast.TypeSpec)
	if !ok {
		return fmt.Errorf("Object named %s is not a type", name)
	}
	return d.renderType(t)
}

func (d *JSONDoc) renderType(typ *ast.TypeSpec) error {
	if t, ok := typ.Type.(*ast.StructType); ok {
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
					return err
				}
				fields = append(fields, field{name, d.typeString(f.Type, name), strings.TrimSpace(f.Comment.Text())})
			}
		}
		d.t.ExecuteTemplate(&d.b, "table", fields)
	}
	return nil
}

func (d *JSONDoc) findObject(name string) *ast.Object {
	for _, pkg := range d.pkg {
		for _, f := range pkg.Files {
			if o := f.Scope.Objects[name]; o != nil {
				return o
			}
		}
	}
	return nil
}

func (d *JSONDoc) typeString(t ast.Expr, name string) string {
	switch t := t.(type) {
	case *ast.ArrayType:
		if !strings.HasSuffix(name, "-element") {
			name = name + "-element"
		}
		return fmt.Sprintf("array of %s", d.typeString(t.Elt, name))
	case *ast.Ident:
		d.renderLater(t.Name, nil)
		return t.Name
	case *ast.StructType:
		if strings.HasSuffix(name, "-element") {
			d.renderLater(name, t)
			return name
		}
		d.renderLater("of "+name, t)
		return fmt.Sprintf("type of %s", name)
	default:
		return fmt.Sprint(t)
	}
}

func (d *JSONDoc) renderLater(name string, t ast.Expr) {
	if t != nil {
		d.renderQueue = append(d.renderQueue, &ast.TypeSpec{Name: &ast.Ident{Name: name}, Type: t})
	}
	if _, present := d.rendered[name]; present {
		return
	}
	if o := d.findObject(name); o != nil {
		if t, ok := o.Decl.(*ast.TypeSpec); ok {
			d.renderQueue = append(d.renderQueue, t)
			d.rendered[name] = struct{}{}
		}
	}
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
