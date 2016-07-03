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

	"github.com/russross/blackfriday"
)

func main() {
	log.SetFlags(0)
	d, err := NewJSONDoc("../../example", "../../example/index.md")
	if err != nil {
		log.Fatal(err)
	}
	if err := d.WriteTo(os.Stdout); err != nil {
		log.Fatal(err)
	}
}

const header = `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
<title>{{.}}</title>
<style>
@media print {
    body {
        margin: 1em;
    }
    nav {
        display: none;
    }
    table, td, th {
        border: solid 1px #000000;
    }
}
@media screen {
    body {
        margin: 0 1em 0 300px;
        border-left: solid 1px #e0e0e0;
        padding-left: 1em;
        padding-top: 1px;
        padding-bottom: 1em;
    }
    nav {
        position: absolute;
        left: 0px;
        top: 0px;
        width: 300px;
        height: 100%;
        float: left;
        font-size: 80%;
        padding-top: 1em;
    }
    nav ul {
        list-style-type:none;
        padding-left: 1em;
    }
    h1, h2, h3 {
        padding-left: 3px;
    }
    h4 {
        padding-left: 2em;
    }
    h2, h3 {
        margin-top: 2em;
        padding-bottom: 3px;
        border-bottom: solid 3px #c5cae9;
    }
    p, table {
        margin-left: 2em;
    }
    table, td, th {
        border: solid 1px #c5cae9;
    }
    a {
        color: #5c6bc0;
    }
    a:visited {
        color: #ab47bc;
    }
}
h1, h2, h3, h4 {
    font-family: sans-serif;
}
table {
    border-collapse: collapse;
    page-break-inside: avoid;
}
td, th {
    padding: 0.7em;
}
th {
    background-color: #e8eaf6;
}
</style>
</head>
<body>
`

var headerTmpl = template.Must(template.New("header").Parse(header))

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
	links       map[string]map[ast.Expr]int
	title       string
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
	d := &JSONDoc{pkgName: pkg, rendered: make(map[string]struct{}), links: make(map[string]map[ast.Expr]int)}
	fset := token.NewFileSet()
	p, err := parser.ParseDir(fset, pkg, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	d.pkg = p
	d.t = template.New("table").Funcs(template.FuncMap{"input": d.input, "output": d.output, "title": d.setTitle})
	if _, err := d.t.Parse(table); err != nil {
		return nil, err
	}
	if _, err := d.t.ParseFiles(index); err != nil {
		return nil, err
	}
	d.tmplName = filepath.Base(index)
	return d, nil
}

const htmlFlags = blackfriday.HTML_TOC

const commonExtensions = 0 |
	blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
	blackfriday.EXTENSION_TABLES |
	blackfriday.EXTENSION_FENCED_CODE |
	blackfriday.EXTENSION_AUTOLINK |
	blackfriday.EXTENSION_STRIKETHROUGH |
	blackfriday.EXTENSION_SPACE_HEADERS |
	blackfriday.EXTENSION_HEADER_IDS |
	blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
	blackfriday.EXTENSION_DEFINITION_LISTS

func (d *JSONDoc) WriteTo(w io.Writer) error {
	var b bytes.Buffer
	if err := d.t.ExecuteTemplate(&b, d.tmplName, nil); err != nil {
		return err
	}
	out := blackfriday.Markdown(b.Bytes(), blackfriday.HtmlRenderer(htmlFlags, "", ""), commonExtensions)
	err := headerTmpl.Execute(w, html.EscapeString(d.title))
	if err == nil {
		_, err = w.Write(out)
	}
	if err == nil {
		_, err = io.WriteString(w, footer)
	}
	return err
}

func (d *JSONDoc) setTitle(title string) string {
	d.title = title
	return ""
}

func (d *JSONDoc) input(name string) (string, error) {
	d.b.Reset()
	fmt.Fprintf(&d.b, "### Input (%s)\n<div>\n", markdownEscapeString(name))
	d.rendered[name] = struct{}{}
	if err := d.renderTypes(name); err != nil {
		return "", err
	}
	d.b.WriteString("</div>\n")
	return d.b.String(), nil
}

func (d *JSONDoc) output(name string) (string, error) {
	d.b.Reset()
	fmt.Fprintf(&d.b, "### Output (%s)\n<div>\n", markdownEscapeString(name))
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
		s := "type-" + strings.Replace(t.Name.Name, " ", "-", -1)
		i := d.links[s][t.Type]
		if i > 0 {
			s = fmt.Sprintf("%s-%d", s, i)
		}
		fmt.Fprintf(&d.b, "<h4 id=\"%s\">Type %s</h4>\n", html.EscapeString(s), html.EscapeString(t.Name.Name))
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

type field struct {
	Name, Type, Description string
}

func (d *JSONDoc) renderType(typ *ast.TypeSpec) error {
	if t, ok := typ.Type.(*ast.StructType); ok {
		fields, err := d.appendFields(nil, t)
		if err != nil {
			return err
		}
		d.t.ExecuteTemplate(&d.b, "table", fields)
	}
	return nil
}

func (d *JSONDoc) appendFields(fields []field, t *ast.StructType) ([]field, error) {
	for _, f := range t.Fields.List {
		if len(f.Names) == 0 {
			o := d.findObject(f.Type.(*ast.Ident).Name)
			if o == nil {
				continue
			}
			t, ok := o.Decl.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if t, ok := t.Type.(*ast.StructType); ok {
				var err error
				fields, err = d.appendFields(fields, t)
				if err != nil {
					return nil, err
				}
			}
		}
		for _, indent := range f.Names {
			name, err := tagToName(indent.Name, f.Tag)
			if err != nil {
				if err == NotExported {
					continue
				}
				return nil, err
			}
			fields = append(fields, field{html.EscapeString(name), d.typeLink(f.Type, name),
				html.EscapeString(strings.TrimSpace(f.Comment.Text()))})
		}
	}
	return fields, nil
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

func (d *JSONDoc) typeLink(t ast.Expr, name string) string {
	switch t := t.(type) {
	case *ast.ArrayType:
		if !strings.HasSuffix(name, "-element") {
			name = name + "-element"
		}
		return fmt.Sprintf("array of %s", d.typeLink(t.Elt, name))
	case *ast.Ident:
		if ID := d.renderLater(t.Name, nil); ID != "" {
			return fmt.Sprintf(`<a href="#%s">%s</a>`, html.EscapeString(ID), html.EscapeString(t.Name))
		}
		return html.EscapeString(t.Name)
	case *ast.StructType:
		if strings.HasSuffix(name, "-element") {
			ID := d.renderLater(name, t)
			return fmt.Sprintf(`<a href="#%s">%s</a>`, html.EscapeString(ID), html.EscapeString(name))
		}
		ID := d.renderLater("of "+name, t)
		return fmt.Sprintf(`<a href="#%s">type of %s</a>`, html.EscapeString(ID), html.EscapeString(name))
	default:
		return html.EscapeString(fmt.Sprint(t))
	}
}

func (d *JSONDoc) renderLater(name string, t ast.Expr) string {
	if t != nil {
		d.renderQueue = append(d.renderQueue, &ast.TypeSpec{Name: &ast.Ident{Name: name}, Type: t})
		s := "type-" + strings.Replace(name, " ", "-", -1)
		if d.links[s] == nil {
			d.links[s] = make(map[ast.Expr]int)
		}
		i := len(d.links[s]) + 1
		d.links[s][t] = i
		return fmt.Sprintf("%s-%d", s, i)
	}
	if _, present := d.rendered[name]; present {
		return "type-" + name
	}
	if o := d.findObject(name); o != nil {
		if t, ok := o.Decl.(*ast.TypeSpec); ok {
			d.renderQueue = append(d.renderQueue, t)
			d.rendered[name] = struct{}{}
			return "type-" + name
		}
	}
	return ""
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

var isASCIIPunctuation [128]bool

func init() {
	for _, c := range "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~." {
		isASCIIPunctuation[c] = true
	}
}

func markdownEscapeString(s string) string {
	var b bytes.Buffer
	for _, c := range s {
		if c < 128 && isASCIIPunctuation[c] {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}
