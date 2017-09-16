// Copyright 2016-2017 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
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
	d, err := NewJSONDoc("../../example/index.md")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := d.WriteTo(os.Stdout); err != nil {
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
    :target {
        color : #1abc9c;
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
	imports      map[string]string       // map: local in template name -> package path
	packages     map[string]*ast.Package // map: package path -> package AST
	packageNames map[string]string       // map: package path -> package name (may be obtained without parsing the package)
	t            *template.Template
	tmplName     string
	b            bytes.Buffer
	rendered     map[renderedElem]string
	renderQueue  []queueElem
	links        map[string]map[ast.Expr]int
	title        string
}

type queueElem struct {
	t  *ast.TypeSpec
	c  *context
	id string
}

type renderedElem struct {
	Name string
	Obj  *ast.Object
}

const table = `
<p>JSON {{.Prefix}}object{{.S}} with the following fields:</p>
<table>
<tr>
<th>Key name</th>
<th>Value type</th>
<th>Description</th>
</tr>
{{range .Fields}}
<tr>
<td>{{.Name}}</td>
<td>{{.Type}}</td>
<td>{{.Description}}</td>
</tr>
{{end}}
</table>
`

func NewJSONDoc(index string) (*JSONDoc, error) {
	d := &JSONDoc{rendered: make(map[renderedElem]string), links: make(map[string]map[ast.Expr]int),
		packages: make(map[string]*ast.Package), packageNames: make(map[string]string), imports: make(map[string]string)}
	d.t = template.New("table").Funcs(template.FuncMap{"input": d.input, "output": d.output, "title": d.setTitle, "import": d.importPkg})
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

func (d *JSONDoc) WriteTo(w io.Writer) (int64, error) {
	var b bytes.Buffer
	if err := d.t.ExecuteTemplate(&b, d.tmplName, nil); err != nil {
		return 0, err
	}
	out := blackfriday.Markdown(b.Bytes(), blackfriday.HtmlRenderer(htmlFlags, "", ""), commonExtensions)
	b.Reset()
	err := headerTmpl.Execute(&b, html.EscapeString(d.title))
	var n, m, o int
	if err == nil {
		n, err = w.Write(b.Bytes())
	}
	if err == nil {
		m, err = w.Write(out)
	}
	if err == nil {
		o, err = io.WriteString(w, footer)
	}
	return int64(n) + int64(m) + int64(o), err
}

func (d *JSONDoc) setTitle(title string) string {
	d.title = title
	return ""
}

func (d *JSONDoc) importPkg(name, path string) (string, error) {
	if d.imports[name] != "" {
		return "", fmt.Errorf("name %s already imported", name)
	}
	if _, err := d.parsedPackage(path); err != nil {
		return "", err
	}
	d.imports[name] = path
	return "", nil
}

func (d *JSONDoc) input(name string) (string, error) {
	d.b.Reset()
	fmt.Fprintf(&d.b, "### Input (%s)\n<div>\n", markdownEscapeString(name))
	if err := d.renderTypes(name); err != nil {
		return "", err
	}
	d.b.WriteString("</div>\n")
	return d.b.String(), nil
}

func (d *JSONDoc) output(name string) (string, error) {
	d.b.Reset()
	ident := name
	if i := strings.LastIndexByte(name, '.'); i != -1 {
		ident = name[i+1:]
	}
	fmt.Fprintf(&d.b, "### Output (%s)\n<div>\n", markdownEscapeString(ident))
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
		q := d.renderQueue[i]
		fmt.Fprintf(&d.b, "<h4 id=\"%s\">Type %s</h4>\n", html.EscapeString(q.id), html.EscapeString(q.t.Name.Name))
		if err := d.renderType(q.t, q.c); err != nil {
			return err
		}
	}
	d.renderQueue = d.renderQueue[:0]
	return nil
}

func (d *JSONDoc) renderTypeByName(name string) error {
	pkgName := "."
	i := strings.LastIndexByte(name, '.')
	if i != -1 {
		pkgName = name[:i]
		name = name[i+1:]
	}
	path := d.imports[pkgName]
	if path == "" {
		return fmt.Errorf("name %s mast be imported to access %s", pkgName, name)
	}
	o, c, err := d.findObject(name, d.packages[path], path)
	if o == nil {
		return fmt.Errorf("Type %s error: %v", name, err)
	}
	t, ok := o.Decl.(*ast.TypeSpec)
	if !ok {
		return fmt.Errorf("Object named %s is not a type", name)
	}
	return d.renderType(t, c)
}

type field struct {
	Name, Type, Description string
}

func (d *JSONDoc) renderType(typ *ast.TypeSpec, c *context) error {
	return d.renderType1(typ.Type, c, "")
}

func (d *JSONDoc) renderType1(typ ast.Expr, c *context, prefix string) error {
	switch t := typ.(type) {
	case *ast.StructType:
		fields, err := d.appendFields(nil, t, c)
		if err != nil {
			return err
		}
		s := ""
		if prefix != "" {
			s = "s"
		}
		if len(fields) > 0 {
			type data struct {
				Prefix, S string
				Fields    []field
			}
			d.t.ExecuteTemplate(&d.b, "table", data{prefix, s, fields})
		} else {
			fmt.Fprintf(&d.b, "<p>JSON %sobject%s with no fields.</p>\n", prefix, s)
		}
	case *ast.MapType:
		if ident, ok := t.Key.(*ast.Ident); ok && ident.Name == "string" {
			if prefix == "" {
				prefix = "object of "
			} else {
				prefix = prefix + " objects of "
			}
			return d.renderType1(t.Value, c, prefix)
		} else {
			return errors.New("only maps with string keys are supported")
		}
	case *ast.ArrayType:
		if prefix == "" {
			prefix = "array of "
		} else {
			prefix = prefix + " arrays of "
		}
		return d.renderType1(t.Elt, c, prefix)
	}
	return nil
}

func (d *JSONDoc) appendFields(fields []field, t *ast.StructType, c *context) ([]field, error) {
	for _, f := range t.Fields.List {
		if len(f.Names) == 0 {
			o, c, err := d.findObject(f.Type.(*ast.Ident).Name, c.Package, c.Path)
			if err != nil {
				return nil, err
			}
			if o == nil {
				continue
			}
			t, ok := o.Decl.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if t, ok := t.Type.(*ast.StructType); ok {
				var err error
				fields, err = d.appendFields(fields, t, c)
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
			fields = append(fields, field{html.EscapeString(name), d.typeLink(f.Type, c, name, ""),
				html.EscapeString(strings.TrimSpace(f.Comment.Text()))})
		}
	}
	return fields, nil
}

type context struct {
	Path    string
	Package *ast.Package
	File    *ast.File
}

func (d *JSONDoc) findObject(name string, pkg *ast.Package, path string) (*ast.Object, *context, error) {
	for _, f := range pkg.Files {
		if o := f.Scope.Objects[name]; o != nil {
			return o, &context{path, pkg, f}, nil
		}
	}
	if builtin[name] {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("identifier %s not found in package %s", name, path)
}

func (d *JSONDoc) parsedPackage(path string) (*ast.Package, error) {
	if pkg := d.packages[path]; pkg != nil {
		return pkg, nil
	}
	p, err := build.Import(path, "", 0)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, filepath.Join(p.SrcRoot, path), notTest, parser.ParseComments)
	if len(pkg) > 1 {
		return nil, fmt.Errorf("more than one package in directory %s", path)
	}
	for _, p := range pkg {
		d.packages[path] = p
		d.packageNames[path] = p.Name
		return p, nil
	}
	return nil, fmt.Errorf("package %s is empty", path)
}

func notTest(info os.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}

var builtin = map[string]bool{
	"bool":       true,
	"byte":       true,
	"complex128": true,
	"complex64":  true,
	"error":      true,
	"float32":    true,
	"float64":    true,
	"int":        true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"int8":       true,
	"rune":       true,
	"string":     true,
	"uint":       true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uint8":      true,
	"uintptr":    true,
}

func (d *JSONDoc) typeLink(t ast.Expr, c *context, name string, suffix string) string {
	switch t := t.(type) {
	case *ast.ArrayType:
		if !strings.HasSuffix(name, "-element") {
			name = name + "-element"
		}
		return fmt.Sprintf("array%s of %s", suffix, d.typeLink(t.Elt, c, name, "s"))
	case *ast.MapType:
		if ident, ok := t.Key.(*ast.Ident); ok && ident.Name == "string" {
			if !strings.HasSuffix(name, "-element") {
				name = name + "-element"
			}
			return fmt.Sprintf("object%s of %s", suffix, d.typeLink(t.Value, c, name, "s"))
		} else {
			return "(error: only maps with string keys are supported)"
		}
	case *ast.Ident:
		if ID := d.renderLater(t.Name, nil, c); ID != "" {
			return fmt.Sprintf(`<a href="#%s">%s</a>`, html.EscapeString(ID), html.EscapeString(t.Name))
		}
		return html.EscapeString(t.Name)
	case *ast.StructType:
		if strings.HasSuffix(name, "-element") {
			ID := d.renderLater(name, t, c)
			return fmt.Sprintf(`<a href="#%s">%s</a>`, html.EscapeString(ID), html.EscapeString(name))
		}
		ID := d.renderLater("of "+name, t, c)
		return fmt.Sprintf(`<a href="#%s">type of %s</a>`, html.EscapeString(ID), html.EscapeString(name))
	case *ast.SelectorExpr:
		ident, ok := t.X.(*ast.Ident)
		if !ok {
			fmt.Fprintf(os.Stderr, "type %v: expected identifier before '.'\n", t)
			return html.EscapeString(fmt.Sprint(t))
		}
		path, err := d.findImportIdent(c.File, ident.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "type %s.%s: %v\n", ident.Name, t.Sel.Name, err)
			return html.EscapeString(fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name))
		}
		pkg, err := d.parsedPackage(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "type %s.%s: %v\n", ident.Name, t.Sel.Name, err)
			return html.EscapeString(fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name))
		}
		_, c, err := d.findObject(t.Sel.Name, pkg, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "type %s.%s: %v\n", ident.Name, t.Sel.Name, err)
			return html.EscapeString(fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name))
		}
		if ID := d.renderLater(t.Sel.Name, nil, c); ID != "" {
			return fmt.Sprintf(`<a href="#%s">%s</a>`, html.EscapeString(ID), html.EscapeString(t.Sel.Name))
		}
		return html.EscapeString(fmt.Sprintf("%s.%s", ident.Name, t.Sel.Name))
	default:
		return html.EscapeString(fmt.Sprint(t))
	}
}

func (d *JSONDoc) findImportIdent(file *ast.File, name string) (string, error) {
	for _, imp := range file.Imports {
		path := imp.Path.Value[1 : len(imp.Path.Value)-1]
		if imp.Name != nil {
			if imp.Name.Name == name {
				return path, nil
			}
			continue
		}
		s := d.packageNames[path]
		if s == "" {
			p, err := build.Import(path, "", 0)
			if err != nil {
				return "", err
			}
			s = p.Name
			d.packageNames[path] = s
		}
		if s == name {
			return path, nil
		}
	}
	return "", fmt.Errorf(`package named %s not found (may need to run "go build -i")`, name)
}

func (d *JSONDoc) renderLater(name string, t ast.Expr, c *context) string {
	if t != nil {
		s := "type-" + strings.Replace(name, " ", "-", -1)
		if d.links[s] == nil {
			d.links[s] = make(map[ast.Expr]int)
		}
		i := len(d.links[s]) + 1
		d.links[s][t] = i
		s = fmt.Sprintf("%s-%d", s, i)
		d.renderQueue = append(d.renderQueue, queueElem{&ast.TypeSpec{Name: &ast.Ident{Name: name}, Type: t}, c, s})
		return s
	}
	o, c, err := d.findObject(name, c.Package, c.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	if o == nil {
		return ""
	}
	if s := d.rendered[renderedElem{name, o}]; s != "" {
		return s
	}
	if t, ok := o.Decl.(*ast.TypeSpec); ok {
		s := "type-" + name
		if d.links[s] == nil {
			d.links[s] = make(map[ast.Expr]int)
		}
		i := len(d.links[s]) + 1
		d.links[s][t.Type] = i
		s = fmt.Sprintf("%s-%d", s, i)
		d.renderQueue = append(d.renderQueue, queueElem{t, c, s})
		d.rendered[renderedElem{name, o}] = s
		return s
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
