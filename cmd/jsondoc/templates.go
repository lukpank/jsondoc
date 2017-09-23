package main

import "text/template"

const htmlHeader = `
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

var htmlHeaderTmpl = template.Must(template.New("header").Parse(htmlHeader))

const htmlFooter = `
</body>
</html>
`

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
