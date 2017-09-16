jsondoc
=======

*jsondoc* is a command line tool used to simplify creation of
documentation of HTTP (REST) JSON APIs for projects written in Go. The
input and/or output JSON structure for particular endpoints is
obtained from named types from selected Go packages. The output of
jsondoc is an HTML file with embedded CSS.


Installation
------------

*jsondoc* can be installed with

```
go get github.com/lukpank/jsondoc/cmd/jsondoc
```


Usage
-----

You run *jsondoc* by specifying input and output files like this

```
$ jsondoc -o output.html input.md
```

where `input.md` is both a markdown file and a Go text template. This
means that you may write your documentation as a markdown document
including some text template actions.


Example
-------

For an example of such a markdown file and corresponding Go package see
[example directory](https://github.com/lukpank/jsondoc/tree/master/example).
HTML output from *jsondoc* for this example is available 
[here](http://lukpank.github.io/jsondoc/example.html).


Description
-----------

You start the document with a title to be used in HTML header 

```
{{title "Example JSON API description"}}
```

So if you want to display it on the page you have to use regular
markdown header

```
# Example JSON API description
```

Then you can "import" packages from which you want to access types
with 

```
{{import "pkg" "package/path"}}
```
	
and then you can access types from this package with `pkg.typeName`
(in particular you can access unexported types). If you want to access
types from some package with just `typeName` you can import this
package in the following way

```
{{import "." "package/path"}}
```

Each name may (such as `pkg` and in particular `.`) may be imported
only once.


Now you can describe each endpoint in the form

```
## Request for path `/hello`

Used to obtain greetings for the given name.

{{input "helloInput"}}

{{output "helloOutput"}}
```

where after the header and short description of the given endpoint
there are two template actions: `input` and `output`, the first one
introduces the Go type representing JSON input of the endpoint and the
second one introduces Go type representing JSON output of the
endpoint. If they are of type `struct` the are presented as HTML
tables, if they have fields with `struct` types they are presented as
tables below the main table. If a field contains `json` struct tag its
name is displayed as "Key name" in the table. If a field contains a
comment it is displayed as "Description" in the table.


Author
------

Łukasz Pankowski (lukpank at o2 dot pl).
